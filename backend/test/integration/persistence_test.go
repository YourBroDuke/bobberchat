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
		DROP TABLE IF EXISTS blacklist_entries, connection_requests, messages_default, messages, chat_group_members, chat_groups, agents, users CASCADE;
		DROP TYPE IF EXISTS connection_request_status, participant_type, group_visibility CASCADE;
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
		DROP TABLE IF EXISTS blacklist_entries, connection_requests, messages_default, messages, chat_group_members, chat_groups, agents, users CASCADE;
		DROP TYPE IF EXISTS connection_request_status, participant_type, group_visibility CASCADE;
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

	created, err := repos.Users.Create(ctx, persistence.User{
		Email:        "user-create-get@example.com",
		PasswordHash: "hashed-password",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := repos.Users.GetByEmail(ctx, "user-create-get@example.com")
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != created.ID {
		t.Errorf("id mismatch: got %s want %s", got.ID, created.ID)
	}
	if got.Email != strings.ToLower("user-create-get@example.com") {
		t.Errorf("email mismatch: got %s", got.Email)
	}
	if got.PasswordHash != "hashed-password" {
		t.Errorf("password hash mismatch: got %s", got.PasswordHash)
	}
}

func TestAgentRepository_CRUD(t *testing.T) {
	db, _ := setupDB(t)
	repos := persistence.NewPostgresRepositories(db)
	ctx := context.Background()

	owner, err := repos.Users.Create(ctx, persistence.User{
		Email:        "agent-owner@example.com",
		PasswordHash: "owner-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	agentInput := persistence.Agent{
		DisplayName:   "integration-agent",
		OwnerUserID:   owner.ID,
		APISecretHash: "secret-hash",
	}

	created, err := repos.Agents.Create(ctx, agentInput)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repos.Agents.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Errorf("agent id mismatch: got %s want %s", got.ID, created.ID)
	}
	if got.OwnerUserID != owner.ID {
		t.Errorf("owner mismatch: got %s want %s", got.OwnerUserID, owner.ID)
	}

	list, err := repos.Agents.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list length mismatch: got %d want 1", len(list))
	}

	if err := repos.Agents.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}

	_, err = repos.Agents.GetByID(ctx, created.ID)
	if !errors.Is(err, persistence.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestChatGroupRepository_CreateAndMembers(t *testing.T) {
	db, _ := setupDB(t)
	repos := persistence.NewPostgresRepositories(db)
	ctx := context.Background()

	creator, err := repos.Users.Create(ctx, persistence.User{
		Email:        "group-creator@example.com",
		PasswordHash: "creator-hash",
	})
	if err != nil {
		t.Fatal(err)
	}

	group, err := repos.Groups.Create(ctx, persistence.ChatGroup{
		Name:    "integration-group",
		OwnerID: creator.ID,
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

	groups, err := repos.Groups.ListAll(ctx)
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
