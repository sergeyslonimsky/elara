package auth_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sergeyslonimsky/elara/internal/auth"
	"github.com/sergeyslonimsky/elara/internal/domain"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

type fakeTokenRepo struct {
	tokens []*domain.PAT
	err    error
}

func (f *fakeTokenRepo) Create(_ context.Context, pat *domain.PAT) error {
	if f.err != nil {
		return f.err
	}
	f.tokens = append(f.tokens, pat)

	return nil
}

func (f *fakeTokenRepo) List(_ context.Context, userEmail string) ([]*domain.PAT, error) {
	if f.err != nil {
		return nil, f.err
	}
	if userEmail == "" {
		return f.tokens, nil
	}
	var result []*domain.PAT
	for _, t := range f.tokens {
		if t.UserEmail == userEmail {
			result = append(result, t)
		}
	}

	return result, nil
}

func (f *fakeTokenRepo) GetByID(_ context.Context, id string) (*domain.PAT, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, t := range f.tokens {
		if t.ID == id {
			return t, nil
		}
	}

	return nil, domain.NewNotFoundError("token", id)
}

func (f *fakeTokenRepo) Delete(_ context.Context, id string) error {
	if f.err != nil {
		return f.err
	}
	for i, t := range f.tokens {
		if t.ID == id {
			f.tokens = append(f.tokens[:i], f.tokens[i+1:]...)

			return nil
		}
	}

	return domain.NewNotFoundError("token", id)
}

func ctxWithClaims(email string) context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{Email: email})
}

func TestCreateTokenUseCase_Execute(t *testing.T) {
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeTokenRepo{err: tc.repoErr}
			uc := authuc.NewCreateTokenUseCase(repo)

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
		wantLen   int
	}{
		{
			name:      "filters by user email",
			userEmail: "user@example.com",
			wantLen:   1,
		},
		{
			name:      "empty email returns all tokens",
			userEmail: "",
			wantLen:   2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeTokenRepo{tokens: tokens}
			uc := authuc.NewListTokensUseCase(repo)
			got, err := uc.Execute(t.Context(), tc.userEmail)
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
		tokens  []*domain.PAT
		wantErr bool
	}{
		{
			name:   "returns existing token",
			id:     "t1",
			tokens: []*domain.PAT{existing},
		},
		{
			name:    "not found returns error",
			id:      "missing",
			tokens:  []*domain.PAT{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeTokenRepo{tokens: tc.tokens}
			uc := authuc.NewGetTokenUseCase(repo)
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
		tokens  []*domain.PAT
		wantErr bool
	}{
		{
			name:   "revokes existing token",
			id:     "t1",
			tokens: []*domain.PAT{{ID: "t1", UserEmail: "user@example.com"}},
		},
		{
			name:    "not found returns error",
			id:      "missing",
			tokens:  []*domain.PAT{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeTokenRepo{tokens: tc.tokens}
			uc := authuc.NewRevokeTokenUseCase(repo)
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
