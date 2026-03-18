package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/adapter"
	"github.com/bobberchat/bobberchat/backend/internal/adapter/a2a"
	grpcadapter "github.com/bobberchat/bobberchat/backend/internal/adapter/grpc"
	"github.com/bobberchat/bobberchat/backend/internal/adapter/mcp"
	"github.com/bobberchat/bobberchat/backend/internal/approval"
	"github.com/bobberchat/bobberchat/backend/internal/auth"
	"github.com/bobberchat/bobberchat/backend/internal/broker"
	"github.com/bobberchat/bobberchat/backend/internal/conversation"
	"github.com/bobberchat/bobberchat/backend/internal/email"
	"github.com/bobberchat/bobberchat/backend/internal/email/azurecs"
	"github.com/bobberchat/bobberchat/backend/internal/email/console"
	"github.com/bobberchat/bobberchat/backend/internal/observability"
	"github.com/bobberchat/bobberchat/backend/internal/persistence"
	"github.com/bobberchat/bobberchat/backend/internal/protocol"
	"github.com/bobberchat/bobberchat/backend/internal/ratelimit"
	"github.com/bobberchat/bobberchat/backend/internal/registry"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

type config struct {
	Server struct {
		ListenAddress       string `mapstructure:"listen_address"`
		ReadTimeoutSeconds  int    `mapstructure:"read_timeout_seconds"`
		WriteTimeoutSeconds int    `mapstructure:"write_timeout_seconds"`
	} `mapstructure:"server"`
	NATS struct {
		URL string `mapstructure:"url"`
	} `mapstructure:"nats"`
	Postgres struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"postgres"`
	Auth struct {
		JWTSecret string `mapstructure:"jwt_secret"`
	} `mapstructure:"auth"`
	Email struct {
		Provider    string `mapstructure:"provider"`
		FromAddress string `mapstructure:"from_address"`
		Azure       struct {
			ConnectionString string `mapstructure:"connection_string"`
		} `mapstructure:"azure"`
		VerificationTokenTTLHours int `mapstructure:"verification_token_ttl_hours"`
	} `mapstructure:"email"`
	Logging struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"logging"`
	Observability struct {
		MetricsPath string `mapstructure:"metrics_path"`
	} `mapstructure:"observability"`
	RateLimits ratelimit.Config `mapstructure:"rate_limits"`
}

type contextKey string

type messagePublisher interface {
	PublishMessage(ctx context.Context, env *protocol.Envelope) error
}

const (
	ctxUserID  contextKey = "user_id"
	ctxRole    contextKey = "role"
	ctxAgentID contextKey = "agent_id"
)

type app struct {
	db           *persistence.DB
	authSvc      *auth.Service
	registrySvc  *registry.Service
	convSvc      *conversation.Service
	approvalSvc  *approval.Service
	broker       *broker.Broker
	publisher    messagePublisher
	adapters     map[string]adapter.Adapter
	metricsReg   *prometheus.Registry
	metrics      *observability.Metrics
	limiter      *ratelimit.Limiter
	auditRepo    persistence.AuditLogRepository
	wsUpgrader   websocket.Upgrader
	heartbeatTTL time.Duration
	activeConns  sync.WaitGroup
}

func main() {
	configPath := flag.String("config", "configs/backend.yaml", "path to backend config")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		panic(err)
	}

	logger := observability.SetupLogger(cfg.Logging.Level, cfg.Logging.Format)
	metricsReg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(metricsReg)

	db, err := persistence.NewDB(cfg.Postgres.DSN)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect postgres")
	}
	defer db.Close()

	brok, err := broker.NewBroker(cfg.NATS.URL, db, metrics)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect nats")
	}
	defer brok.Close()

	if err := brok.Setup(context.Background()); err != nil {
		logger.Fatal().Err(err).Msg("failed to setup jetstream")
	}

	adapters := map[string]adapter.Adapter{
		"mcp":  mcp.NewMCPAdapter(),
		"a2a":  a2a.NewA2AAdapter(),
		"grpc": grpcadapter.NewGRPCAdapter(),
	}

	repos := persistence.NewPostgresRepositories(db)

	var emailSender email.Sender
	switch cfg.Email.Provider {
	case "azure":
		emailSender = azurecs.New(cfg.Email.Azure.ConnectionString, cfg.Email.FromAddress)
	default:
		emailSender = console.New()
	}
	verificationTTL := time.Duration(cfg.Email.VerificationTokenTTLHours) * time.Hour
	if verificationTTL == 0 {
		verificationTTL = 24 * time.Hour
	}

	rlCfg := cfg.RateLimits
	if rlCfg.BurstFactor == 0 && rlCfg.PerAgentMPS == 0 {
		rlCfg = ratelimit.DefaultConfig()
	}

	a := &app{
		db:          db,
		authSvc:     auth.NewService(db, cfg.Auth.JWTSecret, emailSender, verificationTTL),
		registrySvc: registry.NewService(db),
		convSvc:     conversation.NewService(db),
		approvalSvc: approval.NewService(db, brok),
		broker:      brok,
		publisher:   brok,
		adapters:    adapters,
		metricsReg:  metricsReg,
		metrics:     metrics,
		limiter:     ratelimit.New(rlCfg),
		auditRepo:   repos.AuditLogs,
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(_ *http.Request) bool { return true },
		},
		heartbeatTTL: 30 * time.Second,
	}

	logger.Info().Int("count", len(adapters)).Msg("protocol adapters registered")

	mux := http.NewServeMux()
	a.registerRoutes(mux)

	srv := &http.Server{
		Addr:         cfg.Server.ListenAddress,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
	}

	go func() {
		logger.Info().Str("addr", cfg.Server.ListenAddress).Msg("bobberd listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("server stopped unexpectedly")
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info().Msg("shutdown signal received, draining connections")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("graceful shutdown failed")
	}

	done := make(chan struct{})
	go func() {
		a.activeConns.Wait()
		close(done)
	}()
	select {
	case <-done:
		logger.Info().Msg("all connections drained")
	case <-ctx.Done():
		logger.Warn().Msg("connection drain timed out")
	}
}

func loadConfig(path string) (*config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("BOBBERD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if cfg.Server.ListenAddress == "" {
		cfg.Server.ListenAddress = ":8080"
	}
	if cfg.Server.ReadTimeoutSeconds <= 0 {
		cfg.Server.ReadTimeoutSeconds = 15
	}
	if cfg.Server.WriteTimeoutSeconds <= 0 {
		cfg.Server.WriteTimeoutSeconds = 15
	}
	if cfg.Observability.MetricsPath == "" {
		cfg.Observability.MetricsPath = "/v1/metrics"
	}

	return &cfg, nil
}

func (a *app) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/register", a.handleRegister)
	mux.HandleFunc("POST /v1/auth/login", a.handleLogin)
	mux.HandleFunc("POST /v1/auth/verify-email", a.handleVerifyEmail)
	mux.HandleFunc("POST /v1/auth/resend-verification", a.handleResendVerification)
	mux.HandleFunc("GET /v1/auth/me", a.requireJWT(a.handleWhoAmI))

	mux.HandleFunc("POST /v1/agents", a.requireJWT(a.handleCreateAgent))
	mux.HandleFunc("GET /v1/agents/{id}", a.requireAuth(true, true, a.handleGetAgent))
	mux.HandleFunc("DELETE /v1/agents/{id}", a.requireJWT(a.handleDeleteAgent))
	mux.HandleFunc("POST /v1/agents/{id}/rotate-secret", a.requireJWT(a.handleRotateSecret))

	mux.HandleFunc("GET /v1/info/{id}", a.requireAuth(true, true, a.handleEntityInfo))

	mux.HandleFunc("POST /v1/registry/discover", a.requireAuth(true, true, a.handleDiscover))
	mux.HandleFunc("GET /v1/registry/agents", a.requireJWT(a.handleListAgents))

	mux.HandleFunc("POST /v1/groups", a.requireJWT(a.handleCreateGroup))
	mux.HandleFunc("GET /v1/groups", a.requireJWT(a.handleListGroups))
	mux.HandleFunc("POST /v1/groups/{id}/join", a.requireAuth(true, true, a.handleJoinGroup))
	mux.HandleFunc("POST /v1/groups/{id}/leave", a.requireAuth(true, true, a.handleLeaveGroup))

	mux.HandleFunc("GET /v1/messages", a.requireJWT(a.handleMessagesByTraceID))
	mux.HandleFunc("GET /v1/messages/poll", a.requireJWT(a.handlePollMessages))
	mux.HandleFunc("POST /v1/messages/{id}/replay", a.requireJWT(a.handleReplayMessage))

	mux.HandleFunc("POST /v1/connections/request", a.requireJWT(a.handleConnectionRequest))
	mux.HandleFunc("GET /v1/connections/inbox", a.requireJWT(a.handleConnectionInbox))
	mux.HandleFunc("POST /v1/connections/{id}/accept", a.requireJWT(a.handleConnectionAccept))
	mux.HandleFunc("POST /v1/connections/{id}/reject", a.requireJWT(a.handleConnectionReject))
	mux.HandleFunc("POST /v1/blacklist", a.requireJWT(a.handleBlacklist))
	mux.HandleFunc("DELETE /v1/blacklist/{id}", a.requireJWT(a.handleUnblacklist))

	mux.HandleFunc("GET /v1/approvals/pending", a.requireJWT(a.handlePendingApprovals))
	mux.HandleFunc("POST /v1/approvals/{id}/decide", a.requireJWT(a.handleDecideApproval))

	mux.HandleFunc("GET /v1/health", a.handleHealth)
	mux.Handle("GET /v1/metrics", promhttp.HandlerFor(a.metricsReg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /v1/ws/connect", a.handleWebSocket)

	mux.HandleFunc("POST /v1/adapter/{name}/ingest", a.requireAuth(true, true, a.handleAdapterIngest))
	mux.HandleFunc("GET /v1/adapter", a.requireJWT(a.handleListAdapters))
}

func (a *app) requireJWT(next http.HandlerFunc) http.HandlerFunc {
	return a.requireAuth(true, false, next)
}

func (a *app) requireAuth(allowJWT, allowAgentSecret bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, err := a.authenticate(r, allowJWT, allowAgentSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		next(w, r.WithContext(ctx))
	}
}

func (a *app) authenticate(r *http.Request, allowJWT, allowAgentSecret bool) (context.Context, error) {
	if allowJWT {
		token := bearerToken(r.Header.Get("Authorization"))
		if token != "" {
			claims, err := a.authSvc.ValidateJWT(token)
			if err == nil {
				ctx := r.Context()
				ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
				ctx = context.WithValue(ctx, ctxRole, claims.Role)
				return ctx, nil
			}
		}
	}

	if allowAgentSecret {
		agentID := strings.TrimSpace(r.Header.Get("X-Agent-ID"))
		secret := strings.TrimSpace(r.Header.Get("X-API-Secret"))
		if agentID != "" && secret != "" {
			agent, err := a.authSvc.ValidateAPISecret(r.Context(), agentID, secret)
			if err == nil {
				ctx := r.Context()
				ctx = context.WithValue(ctx, ctxUserID, "")
				ctx = context.WithValue(ctx, ctxRole, "agent")
				ctx = context.WithValue(ctx, ctxAgentID, agent.AgentID.String())
				return ctx, nil
			}
		}
	}

	return nil, errors.New("invalid credentials")
}

func (a *app) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := a.authSvc.RegisterUser(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"role":       user.Role,
		"created_at": user.CreatedAt,
	})
}

func (a *app) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, user, err := a.authSvc.LoginUser(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": token,
		"expires_in":   int(time.Hour.Seconds()),
		"user": map[string]any{
			"id":         user.ID,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		},
	})
}

func (a *app) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := a.authSvc.VerifyEmail(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"verified": true,
		"user_id":  user.ID,
		"email":    user.Email,
	})
}

func (a *app) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.authSvc.ResendVerification(r.Context(), req.Email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (a *app) handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(contextString(r.Context(), ctxUserID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user context")
		return
	}

	repos := persistence.NewPostgresRepositories(a.db)
	user, err := repos.Users.GetByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agents, err := repos.Agents.ListByOwner(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    user.Role,
		"agents":  agents,
	})
}

func (a *app) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := contextString(r.Context(), ctxUserID)
	agent, secret, err := a.authSvc.CreateAgent(r.Context(), userID, req.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"agent_id":     agent.AgentID,
		"api_secret":   secret,
		"created_at":   agent.CreatedAt,
		"display_name": agent.DisplayName,
	})
}

func (a *app) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := a.registrySvc.GetAgent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":      agent.AgentID,
		"display_name":  agent.DisplayName,
		"owner_user_id": agent.OwnerUserID,
		"created_at":    agent.CreatedAt,
	})
}

func (a *app) handleEntityInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	uid, err := uuid.Parse(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	repos := persistence.NewPostgresRepositories(a.db)

	agent, err := repos.Agents.GetByID(r.Context(), uid)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"type":          "agent",
			"agent_id":      agent.AgentID,
			"display_name":  agent.DisplayName,
			"owner_user_id": agent.OwnerUserID,
			"created_at":    agent.CreatedAt,
		})
		return
	}

	user, err := repos.Users.GetByID(r.Context(), uid)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"type":           "user",
			"id":             user.ID,
			"email":          user.Email,
			"role":           user.Role,
			"email_verified": user.EmailVerified,
			"created_at":     user.CreatedAt,
		})
		return
	}

	group, err := repos.Groups.GetByID(r.Context(), uid)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"type":        "group",
			"id":          group.ID,
			"name":        group.Name,
			"description": group.Description,
			"visibility":  group.Visibility,
			"creator_id":  group.CreatorID,
			"created_at":  group.CreatedAt,
		})
		return
	}

	writeError(w, http.StatusNotFound, "entity not found")
}

func (a *app) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := a.registrySvc.GetAgent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := a.registrySvc.Deregister(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "agent_id": id})
}

func (a *app) handleRotateSecret(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		GracePeriodSeconds int `json:"grace_period_seconds"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	secret, err := a.authSvc.RotateSecret(r.Context(), id, req.GracePeriodSeconds)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":   id,
		"api_secret": secret,
	})
}

func (a *app) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SupportedTags []string `json:"supported_tags"`
		Limit         int      `json:"limit"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	agents, err := a.registrySvc.Discover(r.Context(), registry.DiscoveryQuery{
		SupportedTags: req.SupportedTags,
		Limit:         req.Limit,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func (a *app) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := a.registrySvc.ListAgents(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func (a *app) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	group, err := a.convSvc.CreateGroup(
		r.Context(),
		req.Name,
		req.Description,
		req.Visibility,
		contextString(r.Context(), ctxUserID),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

func (a *app) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := a.convSvc.ListGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (a *app) handleJoinGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		ParticipantID   string `json:"participant_id"`
		ParticipantKind string `json:"participant_kind"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ParticipantID == "" {
		if contextString(r.Context(), ctxAgentID) != "" {
			req.ParticipantID = contextString(r.Context(), ctxAgentID)
			req.ParticipantKind = string(persistence.ParticipantTypeAgent)
		} else {
			req.ParticipantID = contextString(r.Context(), ctxUserID)
			req.ParticipantKind = string(persistence.ParticipantTypeUser)
		}
	}

	if err := a.convSvc.JoinGroup(r.Context(), id, req.ParticipantID, persistence.ParticipantType(req.ParticipantKind)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"group_id": id, "joined": true})
}

func (a *app) handleLeaveGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		ParticipantID   string `json:"participant_id"`
		ParticipantKind string `json:"participant_kind"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ParticipantID == "" {
		if contextString(r.Context(), ctxAgentID) != "" {
			req.ParticipantID = contextString(r.Context(), ctxAgentID)
			req.ParticipantKind = string(persistence.ParticipantTypeAgent)
		} else {
			req.ParticipantID = contextString(r.Context(), ctxUserID)
			req.ParticipantKind = string(persistence.ParticipantTypeUser)
		}
	}

	if err := a.convSvc.LeaveGroup(r.Context(), id, req.ParticipantID, persistence.ParticipantType(req.ParticipantKind)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"group_id": id, "left": true})
}

func (a *app) handleMessagesByTraceID(w http.ResponseWriter, r *http.Request) {
	traceID := strings.TrimSpace(r.URL.Query().Get("trace_id"))
	if traceID == "" {
		writeError(w, http.StatusBadRequest, "trace_id is required")
		return
	}
	tr, err := uuid.Parse(traceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trace_id")
		return
	}
	msgs, err := persistence.NewPostgresRepositories(a.db).Messages.GetByTraceID(r.Context(), tr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (a *app) handlePollMessages(w http.ResponseWriter, r *http.Request) {
	convRaw := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if convRaw == "" {
		writeError(w, http.StatusBadRequest, "conversation_id is required")
		return
	}
	convID, err := uuid.Parse(convRaw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conversation_id")
		return
	}

	limit := 50
	if limitRaw := strings.TrimSpace(r.URL.Query().Get("limit")); limitRaw != "" {
		parsed, err := strconv.Atoi(limitRaw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = parsed
	}

	var sinceTS *time.Time
	if sinceTSRaw := strings.TrimSpace(r.URL.Query().Get("since_ts")); sinceTSRaw != "" {
		parsed, err := time.Parse(time.RFC3339, sinceTSRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since_ts")
			return
		}
		sinceTS = &parsed
	}

	var sinceID *uuid.UUID
	if sinceIDRaw := strings.TrimSpace(r.URL.Query().Get("since_id")); sinceIDRaw != "" {
		parsed, err := uuid.Parse(sinceIDRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since_id")
			return
		}
		sinceID = &parsed
	}

	msgs, err := persistence.NewPostgresRepositories(a.db).Messages.GetByConversation(r.Context(), convID, limit, sinceTS, sinceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (a *app) handleConnectionRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID string `json:"target_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID, err := uuid.Parse(contextString(r.Context(), ctxUserID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user context")
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(req.TargetID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid target_id")
		return
	}

	created, err := persistence.NewPostgresRepositories(a.db).ConnectionRequests.Create(r.Context(), persistence.ConnectionRequest{
		FromUserID: userID,
		ToUserID:   targetID,
		Status:     persistence.ConnectionRequestStatusPending,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"request": created})
}

func (a *app) handleConnectionInbox(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(contextString(r.Context(), ctxUserID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user context")
		return
	}

	requests, err := persistence.NewPostgresRepositories(a.db).ConnectionRequests.GetPendingForUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"requests": requests})
}

func (a *app) handleConnectionAccept(w http.ResponseWriter, r *http.Request) {
	requestID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := persistence.NewPostgresRepositories(a.db).ConnectionRequests.UpdateStatus(r.Context(), requestID, persistence.ConnectionRequestStatusAccepted); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"request_id": requestID, "status": persistence.ConnectionRequestStatusAccepted})
}

func (a *app) handleConnectionReject(w http.ResponseWriter, r *http.Request) {
	requestID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := persistence.NewPostgresRepositories(a.db).ConnectionRequests.UpdateStatus(r.Context(), requestID, persistence.ConnectionRequestStatusRejected); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"request_id": requestID, "status": persistence.ConnectionRequestStatusRejected})
}

func (a *app) handleBlacklist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID string `json:"target_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID, err := uuid.Parse(contextString(r.Context(), ctxUserID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user context")
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(req.TargetID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid target_id")
		return
	}

	entry, err := persistence.NewPostgresRepositories(a.db).Blacklist.Create(r.Context(), persistence.BlacklistEntry{
		UserID:        userID,
		BlockedUserID: targetID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"entry": entry})
}

func (a *app) handleUnblacklist(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(contextString(r.Context(), ctxUserID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user context")
		return
	}
	targetID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := persistence.NewPostgresRepositories(a.db).Blacklist.Delete(r.Context(), userID, targetID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"removed": true, "target_id": targetID})
}

func (a *app) handleReplayMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	originalMessageID := strings.TrimSpace(r.PathValue("id"))
	originalID, err := uuid.Parse(originalMessageID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid message id")
		return
	}

	var original persistence.Message
	var payloadRaw []byte
	var metadataRaw []byte
	err = a.db.Pool().QueryRow(r.Context(), `
		SELECT id, from_id, conversation_id, tag, payload, metadata, "timestamp", trace_id
		FROM messages
		WHERE id = $1
	`, originalID).Scan(
		&original.ID,
		&original.FromID,
		&original.ConversationID,
		&original.Tag,
		&payloadRaw,
		&metadataRaw,
		&original.Timestamp,
		&original.TraceID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load message")
		return
	}

	original.Payload = map[string]any{}
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &original.Payload); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to decode message payload")
			return
		}
	}

	original.Metadata = map[string]any{}
	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &original.Metadata); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to decode message metadata")
			return
		}
	}

	payload := make(map[string]any, len(original.Payload)+3)
	for k, v := range original.Payload {
		payload[k] = v
	}
	payload["replayed"] = true
	payload["original_message_id"] = original.ID.String()
	payload["replay_reason"] = strings.TrimSpace(req.Reason)

	metadata := make(map[string]any, len(original.Metadata))
	for k, v := range original.Metadata {
		metadata[k] = v
	}

	newMessageID := uuid.NewString()
	newTraceID := uuid.NewString()
	env := &protocol.Envelope{
		ID:        newMessageID,
		From:      original.FromID.String(),
		To:        original.ConversationID.String(),
		Tag:       original.Tag,
		Payload:   payload,
		Metadata:  metadata,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TraceID:   newTraceID,
	}

	if err := a.publishAndAudit(r.Context(), env); err != nil {
		if errors.Is(err, errRateLimited) {
			writeError(w, http.StatusTooManyRequests, "rate limited")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"replayed":            true,
		"new_message_id":      newMessageID,
		"original_message_id": original.ID.String(),
		"trace_id":            newTraceID,
	})
}

func (a *app) handlePendingApprovals(w http.ResponseWriter, r *http.Request) {
	items, err := a.approvalSvc.GetPending(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": items})
}

func (a *app) handleDecideApproval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.approvalSvc.Decide(
		r.Context(),
		id,
		persistence.ApprovalStatus(strings.ToUpper(req.Decision)),
		contextString(r.Context(), ctxUserID),
		req.Reason,
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approval_id": id, "decision": req.Decision})
}

func (a *app) handleAdapterIngest(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	adp, ok := a.adapters[name]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("adapter %q not found", name))
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	defer r.Body.Close()
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "request body is empty")
		return
	}

	meta := adapter.TransportMeta{
		ConnectionID: strings.TrimSpace(r.Header.Get("X-Connection-ID")),
		SourceAddr:   r.RemoteAddr,
		AgentID:      contextString(r.Context(), ctxAgentID),
		Headers:      map[string]string{},
	}
	if target := strings.TrimSpace(r.Header.Get("X-Target-Agent")); target != "" {
		meta.Headers["X-Target-Agent"] = target
	}

	env, err := adp.Ingest(r.Context(), body, meta)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if env.Metadata == nil {
		env.Metadata = map[string]any{}
	}

	if err := a.publishAndAudit(r.Context(), env); err != nil {
		if errors.Is(err, errRateLimited) {
			writeError(w, http.StatusTooManyRequests, "rate limited")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted":   true,
		"adapter":    name,
		"message_id": env.ID,
		"tag":        env.Tag,
		"trace_id":   env.TraceID,
	})
}

func (a *app) handleListAdapters(w http.ResponseWriter, _ *http.Request) {
	result := make([]map[string]string, 0, len(a.adapters))
	for _, adp := range a.adapters {
		result = append(result, map[string]string{
			"name":     adp.Name(),
			"protocol": adp.Protocol(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"adapters": result})
}

func (a *app) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"time":    time.Now().UTC(),
		"version": protocol.ProtocolVersion,
	})
}

func (a *app) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		token = bearerToken(r.Header.Get("Authorization"))
	}
	if token == "" {
		writeError(w, http.StatusUnauthorized, "missing token")
		return
	}

	claims, err := a.authSvc.ValidateJWT(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	conn, err := a.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	a.activeConns.Add(1)
	if a.metrics != nil {
		a.metrics.ActiveWSConns.Inc()
	}
	defer func() {
		a.activeConns.Done()
		if a.metrics != nil {
			a.metrics.ActiveWSConns.Dec()
		}
	}()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	outbound := make(chan *protocol.Envelope, 128)
	if err := a.broker.SubscribeAgent(ctx, claims.UserID, func(env *protocol.Envelope) {
		select {
		case outbound <- env:
		default:
		}
	}); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	var missedPongs int32
	_ = conn.SetReadDeadline(time.Now().Add(a.heartbeatTTL * 3))
	conn.SetPongHandler(func(string) error {
		atomic.StoreInt32(&missedPongs, 0)
		return conn.SetReadDeadline(time.Now().Add(a.heartbeatTTL * 3))
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			var env protocol.Envelope
			if err := conn.ReadJSON(&env); err != nil {
				cancel()
				return
			}
			if env.ID == "" {
				env.ID = uuid.NewString()
			}
			if env.TraceID == "" {
				env.TraceID = uuid.NewString()
			}
			if env.Timestamp == "" {
				env.Timestamp = time.Now().UTC().Format(time.RFC3339)
			}
			if env.Metadata == nil {
				env.Metadata = map[string]any{}
			}
			if env.From == "" {
				env.From = claims.UserID
			}
			if err := env.Validate(); err != nil {
				_ = conn.WriteJSON(map[string]any{"error": err.Error()})
				continue
			}
			if err := a.publishAndAudit(ctx, &env); err != nil {
				_ = conn.WriteJSON(map[string]any{"error": err.Error()})
			}
		}
	}()

	go func() {
		defer wg.Done()
		pingTicker := time.NewTicker(a.heartbeatTTL)
		defer pingTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case env := <-outbound:
				if err := conn.WriteJSON(env); err != nil {
					cancel()
					return
				}
			case <-pingTicker.C:
				if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
					cancel()
					return
				}
				if atomic.AddInt32(&missedPongs, 1) >= 3 {
					cancel()
					return
				}
			}
		}
	}()

	<-ctx.Done()
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"), time.Now().Add(2*time.Second))
	wg.Wait()
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	return nil
}

var (
	errRateLimited = errors.New("rate limited")
)

func (a *app) publishAndAudit(ctx context.Context, env *protocol.Envelope) error {
	if a.limiter != nil {
		agentKey := ratelimit.AgentKey(env.From)
		if !a.limiter.Allow(ratelimit.DimensionAgent, agentKey) {
			if a.metrics != nil {
				a.metrics.RateLimited.WithLabelValues(ratelimit.DimensionAgent, agentKey).Inc()
			}
			return errRateLimited
		}

		if strings.HasPrefix(env.To, "group:") {
			groupKey := ratelimit.GroupKey(strings.TrimPrefix(env.To, "group:"))
			if !a.limiter.Allow(ratelimit.DimensionGroup, groupKey) {
				if a.metrics != nil {
					a.metrics.RateLimited.WithLabelValues(ratelimit.DimensionGroup, groupKey).Inc()
				}
				return errRateLimited
			}
		}

		tagKey := ratelimit.TagKey(env.Tag)
		if !a.limiter.Allow(ratelimit.DimensionTag, tagKey) {
			if a.metrics != nil {
				a.metrics.RateLimited.WithLabelValues(ratelimit.DimensionTag, tagKey).Inc()
			}
			return errRateLimited
		}
	}

	if err := a.publisher.PublishMessage(ctx, env); err != nil {
		return err
	}

	if a.auditRepo != nil {
		fromID, _ := uuid.Parse(env.From)
		toID, _ := uuid.Parse(env.To)
		entry := persistence.AuditLogEntry{
			EventType: "message.published",
			AgentID:   &fromID,
			Details: map[string]any{
				"message_id":  env.ID,
				"from":        env.From,
				"to":          env.To,
				"tag":         env.Tag,
				"trace_id":    env.TraceID,
				"receiver_id": toID.String(),
			},
		}
		_, _ = a.auditRepo.Append(ctx, entry)
		if a.metrics != nil {
			a.metrics.AuditLogged.Inc()
		}
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func bearerToken(header string) string {
	parts := strings.SplitN(strings.TrimSpace(header), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func contextString(ctx context.Context, key contextKey) string {
	v, _ := ctx.Value(key).(string)
	return strings.TrimSpace(v)
}
