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
		name       string
		email      string
		namespaces []string
		role       string
		noAuth     bool
		mock       func(enforcer *auth_mock.MocktokenEnforcer, creator *auth_mock.MocktokenCreator)
		wantErr    bool
	}{
		{
			name:       "creates token with elara_ prefix",
			email:      "user@example.com",
			namespaces: []string{"ns1"},
			role:       "writer",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, creator *auth_mock.MocktokenCreator) {
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "namespace", "read").Return(true, nil)
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "config", "write").Return(true, nil)
				creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:    "no auth context returns unauthorized",
			noAuth:  true,
			wantErr: true,
		},
		{
			name:       "empty namespaces returns error",
			email:      "user@example.com",
			namespaces: []string{},
			wantErr:    true,
		},
		{
			name:       "forbidden namespace returns error",
			email:      "user@example.com",
			namespaces: []string{"ns1"},
			mock: func(enforcer *auth_mock.MocktokenEnforcer, creator *auth_mock.MocktokenCreator) {
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "namespace", "read").Return(false, nil)
			},
			wantErr: true,
		},
		{
			name:       "repo create error propagated",
			email:      "user@example.com",
			namespaces: []string{"ns1"},
			role:       "reader",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, creator *auth_mock.MocktokenCreator) {
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "namespace", "read").Return(true, nil)
				creator.EXPECT().Create(gomock.Any(), gomock.Any()).Return(errors.New("storage error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			creator := auth_mock.NewMocktokenCreator(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, creator)
			}

			uc := authuc.NewCreateTokenUseCase(enforcer, creator)

			ctx := t.Context()
			if !tc.noAuth {
				ctx = ctxWithClaims(tc.email)
			}

			token, rawToken, err := uc.Execute(ctx, "my-token", tc.namespaces, tc.role, nil)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if token.ID == "" {
				t.Error("expected non-empty token ID")
			}

			if token.TokenHash == "" {
				t.Error("expected non-empty token hash")
			}

			if !strings.HasPrefix(rawToken, "elara_") {
				t.Errorf("raw token %q must start with elara_", rawToken)
			}

			if token.IssuedBy != tc.email {
				t.Errorf("got issued by %q, want %q", token.IssuedBy, tc.email)
			}
		})
	}
}

func TestListTokensUseCase_Execute(t *testing.T) {
	t.Parallel()

	tokens := []*domain.Token{
		{ID: "t1", IssuedBy: "other@example.com", Namespaces: []string{"ns1"}},
		{ID: "t2", IssuedBy: "user@example.com", Namespaces: []string{"ns2"}},
		{ID: "t3", IssuedBy: "stranger@example.com", Namespaces: []string{"secret"}},
	}

	tests := []struct {
		name    string
		email   string
		target  string
		mock    func(enforcer *auth_mock.MocktokenEnforcer, lister *auth_mock.MocktokenLister)
		wantLen int
		wantErr bool
	}{
		{
			name:   "admin can see any user tokens",
			email:  "admin@example.com",
			target: "other@example.com",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, lister *auth_mock.MocktokenLister) {
				enforcer.EXPECT().Enforce("admin@example.com", "*", "token", "read").Return(true, nil)
				lister.EXPECT().List(gomock.Any(), "other@example.com").Return(tokens[:1], nil)
			},
			wantLen: 1,
		},
		{
			name:   "user can see own tokens and tokens for their namespaces",
			email:  "user@example.com",
			target: "", // list all they can see
			mock: func(enforcer *auth_mock.MocktokenEnforcer, lister *auth_mock.MocktokenLister) {
				enforcer.EXPECT().Enforce("user@example.com", "*", "token", "read").Return(false, nil)
				lister.EXPECT().List(gomock.Any(), "").Return(tokens, nil)
				// Access to t1's ns1 -> yes
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "namespace", "read").Return(true, nil)
				// t2 is own -> yes (no Enforce call needed due to IssuedBy check)
				// Access to t3's secret -> no
				enforcer.EXPECT().Enforce("user@example.com", "secret", "namespace", "read").Return(false, nil)
			},
			wantLen: 2, // t1 (accessible ns) + t2 (own)
		},
		{
			name:   "user can filter other's tokens if they have namespace access",
			email:  "user@example.com",
			target: "other@example.com",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, lister *auth_mock.MocktokenLister) {
				enforcer.EXPECT().Enforce("user@example.com", "*", "token", "read").Return(false, nil)
				lister.EXPECT().List(gomock.Any(), "other@example.com").Return(tokens[:1], nil)
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "namespace", "read").Return(true, nil)
			},
			wantLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			lister := auth_mock.NewMocktokenLister(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, lister)
			}

			uc := authuc.NewListTokensUseCase(enforcer, lister)
			got, err := uc.Execute(ctxWithClaims(tc.email), tc.target)

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

	existing := &domain.Token{ID: "t1", IssuedBy: "other@example.com", Namespaces: []string{"ns1"}}

	tests := []struct {
		name    string
		email   string
		id      string
		mock    func(enforcer *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter)
		wantErr bool
	}{
		{
			name:  "owner can get token",
			email: "other@example.com",
			id:    "t1",
			mock: func(_ *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(existing, nil)
			},
		},
		{
			name:  "admin can get any token",
			email: "admin@example.com",
			id:    "t1",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(existing, nil)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "token", "read").Return(true, nil)
			},
		},
		{
			name:  "user with namespace access can get token",
			email: "user@example.com",
			id:    "t1",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(existing, nil)
				enforcer.EXPECT().Enforce("user@example.com", "*", "token", "read").Return(false, nil)
				enforcer.EXPECT().Enforce("user@example.com", "ns1", "namespace", "read").Return(true, nil)
			},
		},
		{
			name:  "stranger without namespace access cannot get token",
			email: "stranger@example.com",
			id:    "t1",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(existing, nil)
				enforcer.EXPECT().Enforce("stranger@example.com", "*", "token", "read").Return(false, nil)
				enforcer.EXPECT().Enforce("stranger@example.com", "ns1", "namespace", "read").Return(false, nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			getter := auth_mock.NewMocktokenIDGetter(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, getter)
			}

			uc := authuc.NewGetTokenUseCase(enforcer, getter)
			got, err := uc.Execute(ctxWithClaims(tc.email), tc.id)

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

	existing := &domain.Token{ID: "t1", IssuedBy: "user@example.com"}

	tests := []struct {
		name    string
		email   string
		id      string
		mock    func(enforcer *auth_mock.MocktokenEnforcer, deleter *auth_mock.MocktokenDeleter, getter *auth_mock.MocktokenIDGetter)
		wantErr bool
	}{
		{
			name:  "owner can revoke token",
			email: "user@example.com",
			id:    "t1",
			mock: func(_ *auth_mock.MocktokenEnforcer, deleter *auth_mock.MocktokenDeleter, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(existing, nil)
				deleter.EXPECT().Delete(gomock.Any(), "t1").Return(nil)
			},
		},
		{
			name:  "admin can revoke any token",
			email: "admin@example.com",
			id:    "t1",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, deleter *auth_mock.MocktokenDeleter, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(existing, nil)
				enforcer.EXPECT().Enforce("admin@example.com", "*", "token", "write").Return(true, nil)
				deleter.EXPECT().Delete(gomock.Any(), "t1").Return(nil)
			},
		},
		{
			name:  "stranger cannot revoke token",
			email: "other@example.com",
			id:    "t1",
			mock: func(enforcer *auth_mock.MocktokenEnforcer, _ *auth_mock.MocktokenDeleter, getter *auth_mock.MocktokenIDGetter) {
				getter.EXPECT().GetByID(gomock.Any(), "t1").Return(existing, nil)
				enforcer.EXPECT().Enforce("other@example.com", "*", "token", "write").Return(false, nil)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			enforcer := auth_mock.NewMocktokenEnforcer(ctrl)
			deleter := auth_mock.NewMocktokenDeleter(ctrl)
			getter := auth_mock.NewMocktokenIDGetter(ctrl)

			if tc.mock != nil {
				tc.mock(enforcer, deleter, getter)
			}

			uc := authuc.NewRevokeTokenUseCase(enforcer, deleter, getter)
			err := uc.Execute(ctxWithClaims(tc.email), tc.id)

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
