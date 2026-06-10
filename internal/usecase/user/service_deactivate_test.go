package user_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/service/auth"
	"github.com/sergeyslonimsky/elara/internal/service/auth/casbin"
	"github.com/sergeyslonimsky/elara/internal/service/auth/sessions"
	"github.com/sergeyslonimsky/elara/internal/service/authz"
	grouprepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/group"
	sessionrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/session"
	userrepo "github.com/sergeyslonimsky/elara/internal/storage/bbolt/user"
	"github.com/sergeyslonimsky/elara/internal/usecase/user"
	usermock "github.com/sergeyslonimsky/elara/internal/usecase/user/mocks"
	"github.com/sergeyslonimsky/elara/test/bbolttest"
)

func TestService_Deactivate(t *testing.T) {
	t.Parallel()

	t.Run("successfully deactivate user and revoke sessions", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		targetUUID := uuid.New()
		err := st.users.Create(t.Context(), &domain.User{
			ID:          targetUUID,
			Email:       targetEmail,
			DisplayName: targetEmail,
			Status:      domain.UserStatusActive,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: targetEmail},
			},
		})
		require.NoError(t, err)

		// Create a session for the target user
		sessionRepo := st.sessionRepo
		sessionEventRepo := sessionrepo.NewEventRepository(st.pkgManager)
		sessionSvc := sessions.New(sessionRepo, sessionEventRepo, sessions.RealClock{})

		var session *domain.Session
		err = st.txm.WithTx(t.Context(), func(ctx context.Context) error {
			var err error
			session, err = sessionSvc.Create(ctx, sessions.CreateParams{
				UserID:     targetUUID.String(),
				ClientType: string(domain.ClientTypeWeb),
				IP:         "127.0.0.1",
				UserAgent:  "test-agent",
			})

			return err
		})
		require.NoError(t, err)

		// Deactivate the user
		result, err := st.svc.Deactivate(t.Context(), adminActor(), targetUUID)
		require.NoError(t, err)
		assert.Equal(t, domain.UserStatusDeactivated, result.User.Status)

		// Verify user is updated in database
		dbUser, err := st.users.GetByID(t.Context(), targetUUID)
		require.NoError(t, err)
		assert.Equal(t, domain.UserStatusDeactivated, dbUser.Status)

		// Verify session is revoked
		gotSess, err := sessionRepo.Get(t.Context(), session.ID)
		require.NoError(t, err)
		assert.False(t, gotSess.IsActive())
	})

	t.Run("cannot deactivate your own account", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)
		seedUserWithID(t, st, adminID, adminEmail)

		_, err := st.svc.Deactivate(t.Context(), adminActor(), adminUUID)
		require.Error(t, err)
		assert.True(t, domain.IsValidationError(err))
	})

	t.Run("deactivating system user is rejected", func(t *testing.T) {
		t.Parallel()

		st := setupServiceReal(t)
		seedAdminAll(t, st)

		targetUUID := uuid.New()
		err := st.users.Create(t.Context(), &domain.User{
			ID:          targetUUID,
			Email:       targetEmail,
			DisplayName: targetEmail,
			Status:      domain.UserStatusActive,
			System:      true,
			Identities: []domain.Identity{
				{Provider: domain.ProviderBasic, Subject: targetEmail},
			},
		})
		require.NoError(t, err)

		_, err = st.svc.Deactivate(t.Context(), adminActor(), targetUUID)
		require.ErrorIs(t, err, domain.ErrSystemImmutable)
	})
}

func TestService_Deactivate_Rollback(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	stack := bbolttest.OpenStack(t)
	enforcer, txm := stack.Enforcer, stack.Txm
	groupRepo := grouprepo.NewRepository(stack.PkgManager)

	users := userrepo.NewRepository(stack.PkgManager)
	targetUUID := uuid.New()
	err := users.Create(t.Context(), &domain.User{
		ID:          targetUUID,
		Email:       targetEmail,
		DisplayName: targetEmail,
		Status:      domain.UserStatusActive,
		Identities: []domain.Identity{
			{Provider: domain.ProviderBasic, Subject: targetEmail},
		},
	})
	require.NoError(t, err)

	// Mock sessions service to fail
	mockSessions := usermock.NewMocksessionsService(ctrl)
	mockSessions.EXPECT().
		RevokeAllForUser(gomock.Any(), targetUUID.String(), adminID, gomock.Any()).
		Return(assert.AnError)

	pdp := authz.NewPDP(enforcer)
	pap := authz.NewPAP(enforcer, txm)
	scope := authz.NewScope(pdp, pap, groupRepo)

	userSvc := auth.NewUserService(users)
	svc := user.New(txm, users, userSvc, groupRepo, mockSessions, pdp, pap, scope)

	// Add admin policies so authorization passes
	require.NoError(
		t,
		enforcer.WriteTx(t.Context(), txm, func(ctx context.Context, txe *casbin.TxEnforcer) error {
			return txe.AddPolicy(adminID, domain.DomainAll, string(domain.ObjectAll), string(domain.ActionAll))
		}),
	)

	// Deactivation should fail due to mock sessions error
	_, err = svc.Deactivate(t.Context(), adminActor(), targetUUID)
	require.ErrorIs(t, err, assert.AnError)

	// Verify that user status is STILL Active (rolled back!)
	dbUser, err := users.GetByID(t.Context(), targetUUID)
	require.NoError(t, err)
	assert.Equal(t, domain.UserStatusActive, dbUser.Status)
}
