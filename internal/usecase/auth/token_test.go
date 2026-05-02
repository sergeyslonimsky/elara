package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
	auth_mock "github.com/sergeyslonimsky/elara/internal/usecase/auth/mocks"
)

func ctxWithClaims(email string) context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{Email: email})
}

func TestCreateTokenUseCase_Execute(t *testing.T) { // NOSONAR
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		noAuth  bool
		repoErr error
		wantErr bool
	}{
		{
			name:  "creates token with elara_ prefix",
			email: "user@example.com",
		},
		{
			name:    "no auth context returns unauthorized",
			noAuth:  true,
			wantErr: true,
		},
		{
			name:    "repo create error propagated",
			email:   "user@example.com",
			repoErr: errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			creator := auth_mock.NewMocktokenCreator(ctrl)

			if !tc.noAuth {
				creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(tc.repoErr)
			}

			uc := authuc.NewCreateTokenUseCase(creator)

			ctx := t.Context()
			if !tc.noAuth {
				ctx = ctxWithClaims(tc.email)
			}

			pat, rawToken, err := uc.Execute(ctx, "my-token", []string{"ns1"}, nil)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pat.ID == "" {
				t.Error("expected non-empty PAT ID")
			}

			if pat.TokenHash == "" {
				t.Error("expected non-empty token hash")
			}

			if !strings.HasPrefix(rawToken, "elara_") {
				t.Errorf("raw token %q must start with elara_", rawToken)
			}

			if pat.UserEmail != tc.email {
				t.Errorf("got user email %q, want %q", pat.UserEmail, tc.email)
			}
		})
	}
}

func TestListTokensUseCase_Execute(t *testing.T) {
	t.Parallel()

	tokens := []*domain.PAT{
		{ID: "t1", UserEmail: "user@example.com"},
		{ID: "t2", UserEmail: "other@example.com"},
	}

	tests := []struct {
		name      string
		userEmail string
		retTokens []*domain.PAT
		retErr    error
		wantLen   int
		wantErr   bool
	}{
		{
			name:      "filters by user email",
			userEmail: "user@example.com",
			retTokens: tokens[:1],
			wantLen:   1,
		},
		{
			name:      "empty email returns all tokens",
			userEmail: "",
			retTokens: tokens,
			wantLen:   2,
		},
		{
			name:      "returns empty slice",
			userEmail: "nobody@example.com",
			retTokens: []*domain.PAT{},
			wantLen:   0,
		},
		{
			name:    "repo error propagated",
			retErr:  errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			lister := auth_mock.NewMocktokenLister(ctrl)
			lister.EXPECT().List(gomock.Any(), tc.userEmail).Return(tc.retTokens, tc.retErr)

			uc := authuc.NewListTokensUseCase(lister)
			got, err := uc.Execute(t.Context(), tc.userEmail)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tc.wantLen {
				t.Errorf("got %d tokens, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestGetTokenUseCase_Execute(t *testing.T) {
	t.Parallel()

	existing := &domain.PAT{ID: "t1", UserEmail: "user@example.com"}

	tests := []struct {
		name    string
		id      string
		retPAT  *domain.PAT
		retErr  error
		wantErr bool
	}{
		{
			name:   "returns existing token",
			id:     "t1",
			retPAT: existing,
		},
		{
			name:    "not found returns error",
			id:      "missing",
			retErr:  domain.NewNotFoundError("token", "missing"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			getter := auth_mock.NewMocktokenIDGetter(ctrl)
			getter.EXPECT().GetByID(gomock.Any(), tc.id).Return(tc.retPAT, tc.retErr)

			uc := authuc.NewGetTokenUseCase(getter)
			got, err := uc.Execute(t.Context(), tc.id)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.ID != tc.id {
				t.Errorf("got token ID %q, want %q", got.ID, tc.id)
			}
		})
	}
}

func TestRevokeTokenUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		retErr  error
		wantErr bool
	}{
		{
			name: "revokes existing token",
			id:   "t1",
		},
		{
			name:    "not found returns error",
			id:      "missing",
			retErr:  domain.NewNotFoundError("token", "missing"),
			wantErr: true,
		},
		{
			name:    "storage error propagated",
			id:      "t1",
			retErr:  errors.New("storage error"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			deleter := auth_mock.NewMocktokenDeleter(ctrl)
			deleter.EXPECT().Delete(gomock.Any(), tc.id).Return(tc.retErr)

			uc := authuc.NewRevokeTokenUseCase(deleter)
			err := uc.Execute(t.Context(), tc.id)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
