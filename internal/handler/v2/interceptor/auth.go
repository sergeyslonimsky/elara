package interceptor

//go:generate mockgen -destination=mocks/auth_mock.go -package=interceptor_mock -source=auth.go

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/sergeyslonimsky/elara/internal/authctx"
	"github.com/sergeyslonimsky/elara/internal/domain"
)

const sessionCookieName = "elara_session"

//nolint:gochecknoglobals // explicitly requested by code review CR-5 to optimize allocations
var passwordChangeAllowedProcedures = map[string]struct{}{
	"/elara.profile.v1.ProfileService/ChangePassword": {},
	"/elara.profile.v1.ProfileService/Logout":         {},
	"/elara.profile.v1.ProfileService/Me":             {},
}

type (
	sessionValidator interface {
		Validate(ctx context.Context, id string) (*domain.Session, error)
		Refresh(ctx context.Context, id string) error
	}

	userLookup interface {
		GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	}
)

// AuthInterceptor validates the elara_session cookie (or Authorization Bearer token)
// and injects *domain.Session and *domain.User into the request context.
type AuthInterceptor struct {
	sessions        sessionValidator
	users           userLookup
	skipPermissions bool
}

type AuthInterceptorOption func(*AuthInterceptor)

func WithAuthSkipPermissions(skip bool) AuthInterceptorOption {
	return func(i *AuthInterceptor) {
		i.skipPermissions = skip
	}
}

var _ connect.Interceptor = (*AuthInterceptor)(nil)

// NewAuthInterceptor returns an AuthInterceptor that authenticates all requests.
func NewAuthInterceptor(sessionSvc sessionValidator, users userLookup, opts ...AuthInterceptorOption) *AuthInterceptor {
	i := &AuthInterceptor{sessions: sessionSvc, users: users}
	for _, opt := range opts {
		opt(i)
	}

	return i
}

func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authenticate(ctx, req.Header())
		if err != nil {
			return nil, err
		}

		if err := i.checkPasswordChangeRequired(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}

		return next(ctx, req)
	}
}

func (i *AuthInterceptor) WrapStreamingClient(
	next connect.StreamingClientFunc,
) connect.StreamingClientFunc {
	return next
}

func (i *AuthInterceptor) WrapStreamingHandler(
	next connect.StreamingHandlerFunc,
) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}

		if err := i.checkPasswordChangeRequired(ctx, conn.Spec().Procedure); err != nil {
			return err
		}

		return next(ctx, conn)
	}
}

func (i *AuthInterceptor) injectBypassUser(ctx context.Context) context.Context {
	sess := &domain.Session{
		ID:         "bypass-session",
		UserID:     uuid.Nil.String(),
		ClientType: domain.ClientTypeWeb,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(time.Hour),
	}
	user := &domain.User{
		ID:          uuid.Nil,
		Email:       "local-admin@elara.internal",
		DisplayName: "Local Admin",
		Status:      domain.UserStatusActive,
	}

	return authctx.WithSession(ctx, sess, user)
}

func (i *AuthInterceptor) authenticate(
	ctx context.Context,
	header http.Header,
) (context.Context, error) {
	sessionID := extractSessionID(header)
	if sessionID == "" {
		return i.rejectOrBypass(ctx, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized))
	}

	sess, err := i.sessions.Validate(ctx, sessionID)
	if err != nil {
		return i.rejectOrBypass(ctx, unauthenticatedError(err))
	}

	uid, err := uuid.Parse(sess.UserID)
	if err != nil {
		return i.rejectOrBypass(ctx, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized))
	}

	user, err := i.users.GetByID(ctx, uid)
	if err != nil {
		return i.rejectOrBypass(ctx, connect.NewError(connect.CodeUnauthenticated, domain.ErrUnauthorized))
	}

	if user.Status != domain.UserStatusActive {
		return i.rejectOrBypass(ctx, connect.NewError(connect.CodeUnauthenticated, domain.ErrUserDeactivated))
	}

	// Best-effort refresh: extend LastSeenAt / sliding TTL. Failures are logged
	// and ignored — a refresh error must not fail the authenticated request.
	if err := i.sessions.Refresh(ctx, sessionID); err != nil {
		slog.WarnContext(ctx, "session refresh failed", "session_id", sessionID, "err", err)
	}

	return authctx.WithSession(ctx, sess, user), nil
}

// rejectOrBypass returns the bypass-user context when skipPermissions is set
// (local-dev mode), otherwise propagates the prepared error. Centralizing
// this swap keeps authenticate() free of repeated branch boilerplate.
func (i *AuthInterceptor) rejectOrBypass(ctx context.Context, err error) (context.Context, error) {
	if i.skipPermissions {
		return i.injectBypassUser(ctx), nil
	}

	return ctx, err
}

// extractSessionID resolves the session identifier from the request headers.
// Priority: Authorization Bearer header wins over the elara_session cookie.
func extractSessionID(header http.Header) string {
	if bearer := header.Get("Authorization"); bearer != "" {
		if after, ok := strings.CutPrefix(bearer, "Bearer "); ok && after != "" {
			return after
		}
	}

	req := &http.Request{Header: header}
	if cookie, err := req.Cookie(sessionCookieName); err == nil {
		return cookie.Value
	}

	return ""
}

var (
	errSessionRevoked  = errors.New("session revoked")
	errSessionExpired  = errors.New("session expired")
	errSessionNotFound = errors.New("session not found")
	errUnauthenticated = errors.New("unauthenticated")
)

// unauthenticatedError maps domain session sentinel errors to distinct but
// equally unauthenticated connect errors so clients can act on the reason.
func unauthenticatedError(err error) *connect.Error {
	var mapped error

	switch {
	case errors.Is(err, domain.ErrSessionRevoked):
		mapped = errSessionRevoked
	case errors.Is(err, domain.ErrSessionExpired):
		mapped = errSessionExpired
	case errors.Is(err, domain.ErrSessionNotFound):
		mapped = errSessionNotFound
	default:
		mapped = errUnauthenticated
	}

	return connect.NewError(connect.CodeUnauthenticated, mapped)
}

func (i *AuthInterceptor) checkPasswordChangeRequired(ctx context.Context, procedure string) error {
	user, ok := authctx.UserFromContext(ctx)
	if !ok {
		return nil
	}

	if user.PasswordChangeRequired {
		if _, ok := passwordChangeAllowedProcedures[procedure]; !ok {
			return connect.NewError(connect.CodePermissionDenied, domain.ErrPasswordChangeRequired)
		}
	}

	return nil
}
