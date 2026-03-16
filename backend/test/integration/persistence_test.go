//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
	"github.com/google/uuid"
)

func setupDB(t *testing.T) (*persistence.DB, func()) {
	t.Helper()

	dsn := os.Getenv("BOBBERCHAT_TEST_DSN")
	if dsn == "" {
		t.Fatal("BOBBERCHAT_TEST_DSN is required")
	}

	db, err := persistence.NewDB(dsn)
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	ctx := context.Background()

	// Drop existing schema before re-applying migration (handles pre-populated databases)
	_, _ = db.Pool().Exec(ctx, `
		DROP TABLE IF EXISTS blacklist_entries, connection_requests, audit_log, approval_requests, messages_default, messages, topics, chat_group_members, chat_groups, agents, users CASCADE;
		DROP TYPE IF EXISTS connection_request_status, participant_type, urgency, approval_status, topic_status, group_visibility, agent_status CASCADE;
	`)

	migrationFiles, err := filepath.Glob("../../../migrations/*.sql")
	if err != nil {
		db.Close()
		t.Fatalf("find migrations: %v", err)
	}
	sort.Strings(migrationFiles)

	for _, f := range migrationFiles {
		migrationSQL, err := os.ReadFile(f)
		if err != nil {
			db.Close()
			t.Fatalf("read migration %s: %v", f, err)
		}

		if _, err := db.Pool().Exec(ctx, string(migrationSQL)); err != nil {
			db.Close()
			t.Fatalf("apply migration %s: %v", f, err)
		}
	}

	cleanup := func() {
		cleanupCtx := context.Background()
		_, _ = db.Pool().Exec(cleanupCtx, `
			DROP TABLE IF EXISTS blacklist_entries, connection_requests, audit_log, approval_requests, messages_default, messages, topics, chat_group_members, chat_groups, agents, users CASCADE;
			DROP TYPE IF EXISTS connection_request_status, participant_type, urgency, approval_status, topic_status, group_visibility, agent_status CASCADE;
		`)
		db.Close()
	}

	t.Cleanup(cleanup)

	return db, cleanup
}

func TestUserRepository_CreateAndGetByEmail(t *testing.T) {
	db, _ := setupDB(t)
	repos := persistence.NewPostgresRepositories(db)
	ctx := context.Background()
	tenantID := uuid.New()

	created, err := repos.Users.Create(ctx, persistence.User{
		TenantID:     tenantID,
		Email:        "user-create-get@example.com",
		PasswordHash: "hashed-password",
		Role:         "member",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := repos.Users.GetByEmail(ctx, tenantID, "user-create-get@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != created.ID {
		t.Errorf("id mismatch: got %s want %s", got.ID, created.ID)
	}
	if got.TenantID != tenantID {
		t.Errorf("tenant mismatch: got %s want %s", got.TenantID, tenantID)
	}
	if got.Email != strings.ToLower("user-create-get@example.com") {
		t.Errorf("email mismatch: got %s", got.Email)
	}
	if got.PasswordHash != "hashed-password" {
		t.Errorf("password hash mismatch: got %s", got.PasswordHash)
	}
	if got.Role != "member" {
		t.Errorf("role mismatch: got %s", got.Role)
	}
}

func TestAgentRepository_CRUD(t *testing.T) {
	db, _ := setupDB(t)
	repos := persistence.NewPostgresRepositories(db)
	ctx := context.Background()
	tenantID := uuid.New()

	owner, err := repos.Users.Create(ctx, persistence.User{
		TenantID:     tenantID,
		Email:        "agent-owner@example.com",
		PasswordHash: "owner-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	agentInput := persistence.Agent{
		TenantID:      tenantID,
		DisplayName:   "integration-agent",
		OwnerUserID:   owner.ID,
		Capabilities:  []string{"test"},
		Version:       "1.0.0",
		Status:        persistence.AgentStatusRegistered,
		APISecretHash: "secret-hash",
	}

	created, err := repos.Agents.Create(ctx, agentInput)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repos.Agents.GetByID(ctx, tenantID, created.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentID != created.AgentID {
		t.Errorf("agent id mismatch: got %s want %s", got.AgentID, created.AgentID)
	}
	if got.OwnerUserID != owner.ID {
		t.Errorf("owner mismatch: got %s want %s", got.OwnerUserID, owner.ID)
	}

	if err := repos.Agents.UpdateStatus(ctx, tenantID, created.AgentID, persistence.AgentStatusOnline); err != nil {
		t.Fatal(err)
	}

	updated, err := repos.Agents.GetByID(ctx, tenantID, created.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != persistence.AgentStatusOnline {
		t.Errorf("status mismatch: got %s want %s", updated.Status, persistence.AgentStatusOnline)
	}

	list, err := repos.Agents.ListByTenant(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list length mismatch: got %d want 1", len(list))
	}

	if err := repos.Agents.Delete(ctx, tenantID, created.AgentID); err != nil {
		t.Fatal(err)
	}

	_, err = repos.Agents.GetByID(ctx, tenantID, created.AgentID)
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestChatGroupRepository_CreateAndMembers(t *testing.T) {
	db, _ := setupDB(t)
	repos := persistence.NewPostgresRepositories(db)
	ctx := context.Background()
	tenantID := uuid.New()

	creator, err := repos.Users.Create(ctx, persistence.User{
		TenantID:     tenantID,
		Email:        "group-creator@example.com",
		PasswordHash: "creator-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	description := "integration group"
	group, err := repos.Groups.Create(ctx, persistence.ChatGroup{
		TenantID:    tenantID,
		Name:        "integration-group",
		Description: &description,
		Visibility:  persistence.GroupVisibilityPrivate,
		CreatorID:   creator.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	member := persistence.ChatGroupMember{
		GroupID:         group.ID,
		ParticipantID:   creator.ID,
		ParticipantKind: persistence.ParticipantTypeUser,
		JoinedAt:        time.Now().UTC(),
	}

	if err := repos.Groups.AddMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	var memberCount int
	if err := db.Pool().QueryRow(ctx, `
		SELECT count(*)
		FROM chat_group_members
		WHERE group_id = $1 AND participant_id = $2 AND participant_kind = $3
	`, group.ID, creator.ID, string(persistence.ParticipantTypeUser)).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 1 {
		t.Errorf("expected member count 1, got %d", memberCount)
	}

	if err := repos.Groups.RemoveMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	if err := db.Pool().QueryRow(ctx, `
		SELECT count(*)
		FROM chat_group_members
		WHERE group_id = $1 AND participant_id = $2 AND participant_kind = $3
	`, group.ID, creator.ID, string(persistence.ParticipantTypeUser)).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if memberCount != 0 {
		t.Errorf("expected member count 0 after removal, got %d", memberCount)
	}

	groups, err := repos.Groups.ListByTenant(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Errorf("group list length mismatch: got %d want 1", len(groups))
	}
	if groups[0].ID != group.ID {
		t.Errorf("group id mismatch: got %s want %s", groups[0].ID, group.ID)
	}
}

func TestTopicRepository_CreateAndUpdateStatus(t *testing.T) {
	db, _ := setupDB(t)
	repos := persistence.NewPostgresRepositories(db)
	ctx := context.Background()
	tenantID := uuid.New()

	creator, err := repos.Users.Create(ctx, persistence.User{
		TenantID:     tenantID,
		Email:        "topic-creator@example.com",
		PasswordHash: "creator-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	group, err := repos.Groups.Create(ctx, persistence.ChatGroup{
		TenantID:   tenantID,
		Name:       "topic-group",
		Visibility: persistence.GroupVisibilityPrivate,
		CreatorID:  creator.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := repos.Topics.Create(ctx, persistence.Topic{
		TenantID: tenantID,
		GroupID:  group.ID,
		Subject:  "integration test topic",
		Status:   persistence.TopicStatusOpen,
	})
	if err != nil {
		t.Fatal(err)
	}

	topics, err := repos.Topics.ListByGroup(ctx, tenantID, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 1 {
		t.Errorf("topics list length mismatch: got %d want 1", len(topics))
	}
	if topics[0].ID != created.ID {
		t.Errorf("topic id mismatch: got %s want %s", topics[0].ID, created.ID)
	}

	if err := repos.Topics.UpdateStatus(ctx, tenantID, created.ID, persistence.TopicStatusResolved); err != nil {
		t.Fatal(err)
	}

	updated, err := repos.Topics.GetByID(ctx, tenantID, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != persistence.TopicStatusResolved {
		t.Errorf("topic status mismatch: got %s want %s", updated.Status, persistence.TopicStatusResolved)
	}
}

func TestApprovalRepository_CreateDecide(t *testing.T) {
	db, _ := setupDB(t)
	repos := persistence.NewPostgresRepositories(db)
	ctx := context.Background()
	tenantID := uuid.New()

	owner, err := repos.Users.Create(ctx, persistence.User{
		TenantID:     tenantID,
		Email:        "approval-owner@example.com",
		PasswordHash: "owner-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	agent, err := repos.Agents.Create(ctx, persistence.Agent{
		TenantID:      tenantID,
		DisplayName:   "approval-agent",
		OwnerUserID:   owner.ID,
		Capabilities:  []string{"approval"},
		Version:       "1.0.0",
		Status:        persistence.AgentStatusRegistered,
		APISecretHash: "secret-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	approvalID := uuid.New()
	created, err := repos.Approvals.Create(ctx, persistence.ApprovalRequest{
		ApprovalID:    approvalID,
		TenantID:      tenantID,
		AgentID:       agent.AgentID,
		Action:        "deploy",
		Justification: "integration approval test",
		Urgency:       persistence.UrgencyMedium,
		Status:        persistence.ApprovalStatusPending,
		TimeoutMS:     60000,
	})
	if err != nil {
		t.Fatal(err)
	}

	pending, err := repos.Approvals.GetPending(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Errorf("pending approvals length mismatch: got %d want 1", len(pending))
	}
	if pending[0].ApprovalID != created.ApprovalID {
		t.Errorf("approval id mismatch: got %s want %s", pending[0].ApprovalID, created.ApprovalID)
	}

	decidedAt := time.Now().UTC()
	if err := repos.Approvals.Decide(ctx, tenantID, created.ApprovalID, owner.ID, persistence.ApprovalStatusGranted, decidedAt); err != nil {
		t.Fatal(err)
	}

	pending, err = repos.Approvals.GetPending(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending approvals after decision, got %d", len(pending))
	}

	var storedStatus string
	if err := db.Pool().QueryRow(ctx, `
		SELECT status
		FROM approval_requests
		WHERE tenant_id = $1 AND approval_id = $2
	`, tenantID, created.ApprovalID).Scan(&storedStatus); err != nil {
		t.Fatal(err)
	}
	if storedStatus != string(persistence.ApprovalStatusGranted) {
		t.Errorf("stored approval status mismatch: got %s want %s", storedStatus, persistence.ApprovalStatusGranted)
	}
}
