package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bobberchat/bobberchat/internal/approval"
	"github.com/bobberchat/bobberchat/internal/auth"
	"github.com/bobberchat/bobberchat/internal/broker"
	"github.com/bobberchat/bobberchat/internal/conversation"
	"github.com/bobberchat/bobberchat/internal/observability"
	"github.com/bobberchat/bobberchat/internal/persistence"
	"github.com/bobberchat/bobberchat/internal/protocol"
	"github.com/bobberchat/bobberchat/internal/registry"
	"github.com/gorilla/websocket"
	"github.com/google/uuid"
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
	Logging struct {
		Level  string `mapstructure:"level"`
		Format string `mapstructure:"format"`
	} `mapstructure:"logging"`
	Observability struct {
		MetricsPath string `mapstructure:"metrics_path"`
	} `mapstructure:"observability"`
}

type contextKey string

const (
	ctxTenantID contextKey = "tenant_id"
	ctxUserID   contextKey = "user_id"
	ctxRole     contextKey = "role"
	ctxAgentID  contextKey = "agent_id"
)

type app struct {
	db           *persistence.DB
	authSvc      *auth.Service
	registrySvc  *registry.Service
	convSvc      *conversation.Service
	approvalSvc  *approval.Service
	broker       *broker.Broker
	metricsReg   *prometheus.Registry
	wsUpgrader   websocket.Upgrader
	heartbeatTTL time.Duration
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

	a := &app{
		db:          db,
		authSvc:     auth.NewService(db, cfg.Auth.JWTSecret),
		registrySvc: registry.NewService(db),
		convSvc:     conversation.NewService(db),
		approvalSvc: approval.NewService(db, brok),
		broker:      brok,
		metricsReg:  metricsReg,
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(_ *http.Request) bool { return true },
		},
		heartbeatTTL: 30 * time.Second,
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("graceful shutdown failed")
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

	mux.HandleFunc("POST /v1/agents", a.requireJWT(a.handleCreateAgent))
	mux.HandleFunc("GET /v1/agents/{id}", a.requireJWT(a.handleGetAgent))
	mux.HandleFunc("DELETE /v1/agents/{id}", a.requireJWT(a.handleDeleteAgent))
	mux.HandleFunc("POST /v1/agents/{id}/rotate-secret", a.requireJWT(a.handleRotateSecret))

	mux.HandleFunc("POST /v1/registry/discover", a.requireAuth(true, true, a.handleDiscover))
	mux.HandleFunc("GET /v1/registry/agents", a.requireJWT(a.handleListAgents))

	mux.HandleFunc("POST /v1/groups", a.requireJWT(a.handleCreateGroup))
	mux.HandleFunc("GET /v1/groups", a.requireJWT(a.handleListGroups))
	mux.HandleFunc("POST /v1/groups/{id}/join", a.requireAuth(true, true, a.handleJoinGroup))
	mux.HandleFunc("POST /v1/groups/{id}/leave", a.requireAuth(true, true, a.handleLeaveGroup))
	mux.HandleFunc("GET /v1/groups/{id}/topics", a.requireJWT(a.handleListTopics))
	mux.HandleFunc("POST /v1/groups/{id}/topics", a.requireAuth(true, true, a.handleCreateTopic))

	mux.HandleFunc("GET /v1/messages", a.requireJWT(a.handleMessagesByTraceID))
	mux.HandleFunc("POST /v1/messages/{id}/replay", a.requireJWT(a.handleReplayMessage))

	mux.HandleFunc("GET /v1/approvals/pending", a.requireJWT(a.handlePendingApprovals))
	mux.HandleFunc("POST /v1/approvals/{id}/decide", a.requireJWT(a.handleDecideApproval))

	mux.HandleFunc("GET /v1/health", a.handleHealth)
	mux.Handle("GET /v1/metrics", promhttp.HandlerFor(a.metricsReg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /v1/ws/connect", a.handleWebSocket)
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
				ctx := context.WithValue(r.Context(), ctxTenantID, claims.TenantID)
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
				ctx := context.WithValue(r.Context(), ctxTenantID, agent.TenantID.String())
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
		TenantID string `json:"tenant_id"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := a.authSvc.RegisterUser(r.Context(), req.TenantID, req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         user.ID,
		"tenant_id":  user.TenantID,
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
		"token_type":   "Bearer",
		"expires_in":   int(time.Hour.Seconds()),
		"user": map[string]any{
			"id":         user.ID,
			"tenant_id":  user.TenantID,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		},
	})
}

func (a *app) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName  string   `json:"display_name"`
		Capabilities []string `json:"capabilities"`
		Version      string   `json:"version"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenantID := contextString(r.Context(), ctxTenantID)
	userID := contextString(r.Context(), ctxUserID)
	agent, secret, err := a.authSvc.CreateAgent(r.Context(), tenantID, userID, req.DisplayName, req.Capabilities, req.Version)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"agent_id":    agent.AgentID,
		"api_secret":  secret,
		"status":      agent.Status,
		"created_at":  agent.CreatedAt,
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
	if agent.TenantID.String() != contextString(r.Context(), ctxTenantID) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent_id":       agent.AgentID,
		"tenant_id":      agent.TenantID,
		"display_name":   agent.DisplayName,
		"owner_user_id":  agent.OwnerUserID,
		"capabilities":   agent.Capabilities,
		"version":        agent.Version,
		"status":         agent.Status,
		"connected_at":   agent.ConnectedAt,
		"last_heartbeat": agent.LastHeartbeat,
		"created_at":     agent.CreatedAt,
	})
}

func (a *app) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := a.registrySvc.GetAgent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if agent.TenantID.String() != contextString(r.Context(), ctxTenantID) {
		writeError(w, http.StatusForbidden, "forbidden")
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
		Capability    string   `json:"capability"`
		SupportedTags []string `json:"supported_tags"`
		Status        []string `json:"status"`
		Limit         int      `json:"limit"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	agents, err := a.registrySvc.Discover(r.Context(), contextString(r.Context(), ctxTenantID), registry.DiscoveryQuery{
		Capability:    req.Capability,
		SupportedTags: req.SupportedTags,
		Status:        req.Status,
		Limit:         req.Limit,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func (a *app) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := a.registrySvc.ListAgents(r.Context(), contextString(r.Context(), ctxTenantID))
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
		contextString(r.Context(), ctxTenantID),
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
	groups, err := a.convSvc.ListGroups(r.Context(), contextString(r.Context(), ctxTenantID))
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

func (a *app) handleListTopics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	topics, err := a.convSvc.ListTopics(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"topics": topics})
}

func (a *app) handleCreateTopic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Subject       string  `json:"subject"`
		ParentTopicID *string `json:"parent_topic_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	topic, err := a.convSvc.CreateTopic(r.Context(), contextString(r.Context(), ctxTenantID), id, req.Subject, req.ParentTopicID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, topic)
}

func (a *app) handleMessagesByTraceID(w http.ResponseWriter, r *http.Request) {
	traceID := strings.TrimSpace(r.URL.Query().Get("trace_id"))
	if traceID == "" {
		writeError(w, http.StatusBadRequest, "trace_id is required")
		return
	}
	tid, err := uuid.Parse(contextString(r.Context(), ctxTenantID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant context")
		return
	}
	tr, err := uuid.Parse(traceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trace_id")
		return
	}
	msgs, err := persistence.NewPostgresRepositories(a.db).Messages.GetByTraceID(r.Context(), tid, tr)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

func (a *app) handleReplayMessage(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}

func (a *app) handlePendingApprovals(w http.ResponseWriter, r *http.Request) {
	items, err := a.approvalSvc.GetPending(r.Context(), contextString(r.Context(), ctxTenantID))
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	outbound := make(chan *protocol.Envelope, 128)
	if err := a.broker.SubscribeAgent(ctx, claims.TenantID, claims.UserID, func(env *protocol.Envelope) {
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
			env.Metadata["tenant_id"] = claims.TenantID
			if env.From == "" {
				env.From = claims.UserID
			}
			if err := env.Validate(); err != nil {
				_ = conn.WriteJSON(map[string]any{"error": err.Error()})
				continue
			}
			if err := a.broker.PublishMessage(ctx, &env); err != nil {
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
