package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bobberchat/bobberchat/internal/persistence"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

const jwtDefaultTTL = time.Hour

type Service struct {
	db        *persistence.DB
	jwtSecret []byte
	hashCost  int
}

type JWTClaims struct {
	UserID   string
	TenantID string
	Role     string
}

type jwtTokenClaims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type oldSecretEntry struct {
	Hash      string
	ValidTill time.Time
}

var rotatedSecrets struct {
	mu      sync.RWMutex
	entries map[string][]oldSecretEntry
}

func init() {
	rotatedSecrets.entries = make(map[string][]oldSecretEntry)
}

func NewService(db *persistence.DB, jwtSecret string) *Service {
	return &Service{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		hashCost:  bcrypt.DefaultCost,
	}
}

func (s *Service) RegisterUser(ctx context.Context, tenantID, email, password string) (*persistence.User, error) {
	if s == nil || s.db == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(email) == "" || password == "" {
		return nil, persistence.ErrInvalidInput
	}

	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("parse tenant id: %w", err)
	}

	repos := persistence.NewPostgresRepositories(s.db)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.hashCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	created, err := repos.Users.Create(ctx, persistence.User{
		TenantID:     tid,
		Email:        strings.ToLower(strings.TrimSpace(email)),
		PasswordHash: string(hash),
		Role:         "member",
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) LoginUser(ctx context.Context, email, password string) (accessToken string, user *persistence.User, err error) {
	if s == nil || s.db == nil || strings.TrimSpace(email) == "" || password == "" {
		return "", nil, persistence.ErrInvalidInput
	}

	u, err := s.getUserByEmailAnyTenant(ctx, email)
	if err != nil {
		return "", nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, errors.New("invalid credentials")
	}

	now := time.Now().UTC()
	claims := jwtTokenClaims{
		UserID:   u.ID.String(),
		TenantID: u.TenantID.String(),
		Role:     u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user:" + u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(jwtDefaultTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", nil, fmt.Errorf("sign token: %w", err)
	}

	return signed, u, nil
}

func (s *Service) ValidateJWT(tokenStr string) (*JWTClaims, error) {
	if s == nil || len(s.jwtSecret) == 0 {
		return nil, errors.New("auth service not configured")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &jwtTokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*jwtTokenClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return &JWTClaims{UserID: claims.UserID, TenantID: claims.TenantID, Role: claims.Role}, nil
}

func (s *Service) CreateAgent(ctx context.Context, tenantID, ownerUserID, displayName string, capabilities []string, version string) (agent *persistence.Agent, apiSecret string, err error) {
	if s == nil || s.db == nil || tenantID == "" || ownerUserID == "" || displayName == "" || version == "" {
		return nil, "", persistence.ErrInvalidInput
	}

	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, "", err
	}
	ownerID, err := uuid.Parse(ownerUserID)
	if err != nil {
		return nil, "", err
	}

	secret, err := generateSecret(32)
	if err != nil {
		return nil, "", err
	}
	hash, err := hashSecretArgon2id(secret)
	if err != nil {
		return nil, "", err
	}

	repos := persistence.NewPostgresRepositories(s.db)
	created, err := repos.Agents.Create(ctx, persistence.Agent{
		TenantID:      tid,
		OwnerUserID:   ownerID,
		DisplayName:   displayName,
		Capabilities:  capabilities,
		Version:       version,
		Status:        persistence.AgentStatusRegistered,
		APISecretHash: hash,
	})
	if err != nil {
		return nil, "", err
	}

	return created, secret, nil
}

func (s *Service) ValidateAPISecret(ctx context.Context, agentID, secret string) (*persistence.Agent, error) {
	if s == nil || s.db == nil || agentID == "" || secret == "" {
		return nil, persistence.ErrInvalidInput
	}

	agent, err := s.getAgentByID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if verifySecretArgon2id(agent.APISecretHash, secret) {
		return agent, nil
	}

	if s.validateAgainstGraceSecrets(agentID, secret) {
		return agent, nil
	}

	return nil, errors.New("invalid api secret")
}

func (s *Service) RotateSecret(ctx context.Context, agentID string, gracePeriodSec int) (newSecret string, err error) {
	if s == nil || s.db == nil || agentID == "" {
		return "", persistence.ErrInvalidInput
	}

	a, err := s.getAgentByID(ctx, agentID)
	if err != nil {
		return "", err
	}

	secret, err := generateSecret(32)
	if err != nil {
		return "", err
	}
	newHash, err := hashSecretArgon2id(secret)
	if err != nil {
		return "", err
	}

	_, err = s.db.Pool().Exec(ctx, `UPDATE agents SET api_secret_hash = $2 WHERE agent_id = $1`, a.AgentID, newHash)
	if err != nil {
		return "", err
	}

	if gracePeriodSec > 0 {
		s.storeOldSecret(agentID, a.APISecretHash, time.Now().UTC().Add(time.Duration(gracePeriodSec)*time.Second))
	}

	return secret, nil
}

func (s *Service) getUserByEmailAnyTenant(ctx context.Context, email string) (*persistence.User, error) {
	if s.db == nil || s.db.Pool() == nil {
		return nil, persistence.ErrInvalidInput
	}

	row := s.db.Pool().QueryRow(ctx, `
		SELECT id, tenant_id, email, password_hash, role, created_at
		FROM users
		WHERE email = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, strings.ToLower(strings.TrimSpace(email)))

	u := persistence.User{}
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Service) getAgentByID(ctx context.Context, agentID string) (*persistence.Agent, error) {
	id, err := uuid.Parse(agentID)
	if err != nil {
		return nil, err
	}

	row := s.db.Pool().QueryRow(ctx, `
		SELECT agent_id, tenant_id, display_name, owner_user_id, capabilities, version, status, api_secret_hash, connected_at, last_heartbeat, created_at
		FROM agents
		WHERE agent_id = $1
	`, id)

	a := persistence.Agent{}
	var status string
	if err := row.Scan(&a.AgentID, &a.TenantID, &a.DisplayName, &a.OwnerUserID, &a.Capabilities, &a.Version, &status, &a.APISecretHash, &a.ConnectedAt, &a.LastHeartbeat, &a.CreatedAt); err != nil {
		return nil, err
	}
	a.Status = persistence.AgentStatus(status)
	return &a, nil
}

func (s *Service) storeOldSecret(agentID, hash string, validTill time.Time) {
	rotatedSecrets.mu.Lock()
	defer rotatedSecrets.mu.Unlock()

	entries := rotatedSecrets.entries[agentID]
	now := time.Now().UTC()
	filtered := make([]oldSecretEntry, 0, len(entries)+1)
	for _, e := range entries {
		if e.ValidTill.After(now) {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, oldSecretEntry{Hash: hash, ValidTill: validTill})
	rotatedSecrets.entries[agentID] = filtered
}

func (s *Service) validateAgainstGraceSecrets(agentID, secret string) bool {
	rotatedSecrets.mu.Lock()
	defer rotatedSecrets.mu.Unlock()

	now := time.Now().UTC()
	entries := rotatedSecrets.entries[agentID]
	kept := make([]oldSecretEntry, 0, len(entries))
	matched := false
	for _, e := range entries {
		if !e.ValidTill.After(now) {
			continue
		}
		if verifySecretArgon2id(e.Hash, secret) {
			matched = true
		}
		kept = append(kept, e)
	}
	if len(kept) == 0 {
		delete(rotatedSecrets.entries, agentID)
	} else {
		rotatedSecrets.entries[agentID] = kept
	}
	return matched
}

func generateSecret(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashSecretArgon2id(secret string) (string, error) {
	if secret == "" {
		return "", errors.New("secret is empty")
	}

	const (
		timeCost    uint32 = 3
		memoryCost  uint32 = 64 * 1024
		parallelism uint8  = 2
		hashLen     uint32 = 32
		saltLen            = 16
	)

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(secret), salt, timeCost, memoryCost, parallelism, hashLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memoryCost, timeCost, parallelism, b64Salt, b64Hash), nil
}

func verifySecretArgon2id(encodedHash, secret string) bool {
	memory, iterations, parallelism, salt, hash, err := decodeArgon2Hash(encodedHash)
	if err != nil {
		return false
	}
	otherHash := argon2.IDKey([]byte(secret), salt, iterations, memory, parallelism, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, otherHash) == 1
}

func decodeArgon2Hash(encodedHash string) (memory uint32, iterations uint32, parallelism uint8, salt []byte, hash []byte, err error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return 0, 0, 0, nil, nil, errors.New("invalid argon2 hash format")
	}
	if parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, errors.New("not argon2id hash")
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return
	}
	if version != argon2.Version {
		err = errors.New("incompatible argon2 version")
		return
	}

	kv := strings.Split(parts[3], ",")
	if len(kv) != 3 {
		err = errors.New("invalid argon2 params")
		return
	}

	m, e := strconv.ParseUint(strings.TrimPrefix(kv[0], "m="), 10, 32)
	if e != nil {
		err = e
		return
	}
	t, e := strconv.ParseUint(strings.TrimPrefix(kv[1], "t="), 10, 32)
	if e != nil {
		err = e
		return
	}
	p, e := strconv.ParseUint(strings.TrimPrefix(kv[2], "p="), 10, 8)
	if e != nil {
		err = e
		return
	}

	memory = uint32(m)
	iterations = uint32(t)
	parallelism = uint8(p)

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return
	}
	hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	return
}
