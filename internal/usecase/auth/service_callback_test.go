package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	authuc "github.com/sergeyslonimsky/elara/internal/usecase/auth"
)

func TestService_Callback(t *testing.T) {
	t.Parallel()

	const (
		provider = "oidc"
		subject  = "sub-123"
		email    = "user@example.com"
		nonce    = "nonce-123"
		code     = "code-123"
	)

	userID := uuid.New()

	tests := []struct {
		name     string
		params   authuc.CallbackParams
		mockFunc func(*gomock.Controller) *authuc.Service
		errIs    error
		wantErr  string
		wantUser *domain.User
	}{
		{
			name:   "success fast path",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: true}
				user := &domain.User{
					ID:     userID,
					Email:  email,
					Status: domain.UserStatusActive,
					Identities: []domain.Identity{
						{Provider: domain.ProviderOIDC, Subject: subject},
					},
				}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(user, nil)
				m.users.EXPECT().RecordLogin(gomock.Any(), userID).Return(user, nil)
				m.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&domain.Session{ID: "sess-123"}, nil)

				return svc
			},
			wantUser: &domain.User{ID: userID, Email: email},
		},
		{
			// Exercises the BootstrapOIDC adoption path: the placeholder is
			// System=true with no identities yet. The use case must NOT gate
			// on System=true before reaching UserService.LinkIdentity, which
			// is the layer that owns the placeholder-allowance invariant.
			name:   "success email-fallback link adopts placeholder system user",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: true}
				userNoIdent := &domain.User{
					ID:     userID,
					Email:  email,
					Status: domain.UserStatusActive,
					System: true,
				}
				userLinked := &domain.User{
					ID:     userID,
					Email:  email,
					Status: domain.UserStatusActive,
					System: true,
					Identities: []domain.Identity{
						{Provider: domain.ProviderOIDC, Subject: subject},
					},
				}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(nil, domain.ErrNotFound)
				m.users.EXPECT().GetByEmail(gomock.Any(), email).Return(userNoIdent, nil)
				m.users.EXPECT().LinkIdentity(gomock.Any(), userID, domain.Identity{
					Provider: domain.ProviderOIDC,
					Subject:  subject,
				}).Return(userLinked, nil)
				m.users.EXPECT().RecordLogin(gomock.Any(), userID).Return(userLinked, nil)
				m.sessions.EXPECT().Create(gomock.Any(), gomock.Any()).Return(&domain.Session{ID: "sess-123"}, nil)

				return svc
			},
			wantUser: &domain.User{ID: userID, Email: email},
		},
		{
			name:   "email_verified=false rejects",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: false}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(nil, domain.ErrNotFound)

				return svc
			},
			errIs: domain.ErrIdentityNotProvisioned,
		},
		{
			name:   "empty email_claim rejects",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: "", EmailVerified: true}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(nil, domain.ErrNotFound)

				return svc
			},
			errIs: domain.ErrIdentityNotProvisioned,
		},
		{
			name:   "email-fallback user not found rejects",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: true}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(nil, domain.ErrNotFound)
				m.users.EXPECT().GetByEmail(gomock.Any(), email).Return(nil, domain.ErrNotFound)

				return svc
			},
			errIs: domain.ErrIdentityNotProvisioned,
		},
		{
			name:   "anti-hijack: candidate already has an identity for this provider",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: true}
				userWithOtherSub := &domain.User{
					ID:    userID,
					Email: email,
					Identities: []domain.Identity{
						{Provider: domain.ProviderOIDC, Subject: "other-sub"},
					},
				}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(nil, domain.ErrNotFound)
				m.users.EXPECT().GetByEmail(gomock.Any(), email).Return(userWithOtherSub, nil)

				return svc
			},
			errIs: domain.ErrIdentityNotProvisioned,
		},
		{
			name:   "deactivated user rejected",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: true}
				user := &domain.User{
					ID:     userID,
					Email:  email,
					Status: domain.UserStatusDeactivated,
					Identities: []domain.Identity{
						{Provider: domain.ProviderOIDC, Subject: subject},
					},
				}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(user, nil)

				return svc
			},
			errIs: domain.ErrUserDeactivated,
		},
		{
			name:   "Provider.Exchange error short-circuits",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(nil, errors.New("exchange failed"))

				return svc
			},
			wantErr: "exchange code",
		},
		{
			name:   "WithTx propagates inner errors (RecordLogin)",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)
				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: true}
				user := &domain.User{
					ID:     userID,
					Email:  email,
					Status: domain.UserStatusActive,
					Identities: []domain.Identity{
						{Provider: domain.ProviderOIDC, Subject: subject},
					},
				}

				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil)
				m.txm.EXPECT().
					WithTx(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
						return fn(ctx)
					})
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(user, nil)
				m.users.EXPECT().RecordLogin(gomock.Any(), userID).Return(nil, errors.New("record login failed"))

				return svc
			},
			wantErr: "record login",
		},
		{
			name:   "concurrent linking race",
			params: authuc.CallbackParams{Code: code, Nonce: nonce},
			mockFunc: func(ctrl *gomock.Controller) *authuc.Service {
				svc, m := setupService(t, ctrl)

				var mu sync.Mutex
				identities := []domain.Identity{}

				identity := &auth.Identity{Subject: subject, Email: email, EmailVerified: true}
				userBase := &domain.User{ID: userID, Email: email, Status: domain.UserStatusActive}

				// Both calls will exchange successfully
				m.provider.EXPECT().Exchange(gomock.Any(), code, nonce).Return(identity, nil).Times(2)

				// Use a serialized txm to simulate sequential execution within WithTx
				stxm := &serializedTxm{}
				m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).DoAndReturn(stxm.WithTx).Times(2)

				// Both calls miss GetByIdentity initially (the second one will too because we haven't linked yet)
				m.users.EXPECT().GetByIdentity(gomock.Any(), provider, subject).Return(nil, domain.ErrNotFound).Times(2)

				// GetByEmail will return stateful identities
				m.users.EXPECT().
					GetByEmail(gomock.Any(), email).
					DoAndReturn(func(ctx context.Context, e string) (*domain.User, error) {
						mu.Lock()
						defer mu.Unlock()
						u := *userBase
						u.Identities = append([]domain.Identity(nil), identities...)

						return &u, nil
					}).
					Times(2)

				// LinkIdentity should ONLY be called once
				m.users.EXPECT().LinkIdentity(gomock.Any(), userID, domain.Identity{
					Provider: domain.ProviderOIDC,
					Subject:  subject,
				}).DoAndReturn(func(ctx context.Context, id uuid.UUID, ident domain.Identity) (*domain.User, error) {
					mu.Lock()
					defer mu.Unlock()
					identities = append(identities, ident)
					u := *userBase
					u.Identities = identities

					return &u, nil
				}).Times(1)

				// Only one call proceeds to RecordLogin and Create Session
				m.users.EXPECT().RecordLogin(gomock.Any(), userID).Return(userBase, nil).Times(1)
				m.sessions.EXPECT().
					Create(gomock.Any(), gomock.Any()).
					Return(&domain.Session{ID: "sess-race"}, nil).
					Times(1)

				return svc
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			sut := tt.mockFunc(ctrl)

			if tt.name == "concurrent linking race" {
				var wg sync.WaitGroup
				wg.Add(2)
				errs := make([]error, 2)
				for i := range 2 {
					go func(idx int) {
						defer wg.Done()
						_, _, errs[idx] = sut.Callback(t.Context(), tt.params)
					}(i)
				}
				wg.Wait()

				// One should succeed, one should fail with ErrIdentityNotProvisioned
				successCount := 0
				failCount := 0
				for _, err := range errs {
					if err == nil {
						successCount++
					} else if errors.Is(err, domain.ErrIdentityNotProvisioned) {
						failCount++
					}
				}
				assert.Equal(t, 1, successCount, "expected one call to succeed")
				assert.Equal(t, 1, failCount, "expected one call to fail with ErrIdentityNotProvisioned")

				return
			}

			user, sess, err := sut.Callback(t.Context(), tt.params)

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.NotNil(t, sess)

			if tt.wantUser != nil {
				assert.Equal(t, tt.wantUser.ID, user.ID)
				assert.Equal(t, tt.wantUser.Email, user.Email)
			}
		})
	}
}

type serializedTxm struct {
	mu sync.Mutex
}

func (s *serializedTxm) WithTx(ctx context.Context, fn func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fn(ctx)
}
