package approval

import (
	"context"
	"errors"
	"testing"

	"github.com/bobberchat/bobberchat/backend/internal/persistence"
)

func TestSubmitRequest_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "nil service",
			err: func() error {
				var s *Service
				return s.SubmitRequest(context.Background(), &persistence.ApprovalRequest{})
			}(),
		},
		{
			name: "nil request",
			err:  (&Service{}).SubmitRequest(context.Background(), nil),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.err, persistence.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", tc.err)
			}
		})
	}
}

func TestApprovalMethods_InvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "GetPending nil db",
			err: func() error {
				_, err := (&Service{}).GetPending(context.Background())
				return err
			}(),
		},
		{
			name: "Decide empty approvalID",
			err:  (&Service{}).Decide(context.Background(), "", persistence.ApprovalStatusGranted, "approver", "reason"),
		},
		{
			name: "Decide empty approverID",
			err:  (&Service{}).Decide(context.Background(), "approval", persistence.ApprovalStatusGranted, "", "reason"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.err, persistence.ErrInvalidInput) {
				t.Fatalf("expected ErrInvalidInput, got %v", tc.err)
			}
		})
	}
}
