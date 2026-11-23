package token_test

import (
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// usernameWithPrefix matches a generated Username for the given, already
// valid, prefix: "<prefix>-" followed by six lowercase base32 characters.
func usernameWithPrefix(prefix string) gomock.Matcher {
	re := regexp.MustCompile("^" + regexp.QuoteMeta(prefix) + "-[a-z2-7]{6}$")

	return gomock.Cond(func(username string) bool { return re.MatchString(username) })
}

func TestService_EnsureUsernames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mockFunc func(m mocks)
		errIs    error
		wantErr  string
		want     int
	}{
		{
			name: "no tokens",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return(nil, nil)
			},
			want: 0,
		},
		{
			name: "tokens with a username are skipped",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return([]*domain.Token{
					{ID: "t1", Name: "ci", Username: "ci-abcdef"},
					{ID: "t2", Name: "deploy", Username: "deploy-ghijkl"},
				}, nil)
			},
			want: 0,
		},
		{
			// Depends on domain.SanitizeTokenNamePrefix returning an already
			// valid prefix unchanged.
			name: "legacy token with a valid name keeps it as the prefix",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return([]*domain.Token{
					{ID: "t1", Name: "ci"},
					{ID: "t2", Name: "deploy", Username: "deploy-ghijkl"},
				}, nil)
				m.store.EXPECT().SetUsername(gomock.Any(), "t1", usernameWithPrefix("ci")).Return(nil)
			},
			want: 1,
		},
		{
			name: "every legacy token is assigned",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return([]*domain.Token{
					{ID: "t1", Name: "ci"},
					{ID: "t2", Name: "deploy"},
				}, nil)
				m.store.EXPECT().SetUsername(gomock.Any(), "t1", gomock.Any()).Return(nil)
				m.store.EXPECT().SetUsername(gomock.Any(), "t2", gomock.Any()).Return(nil)
			},
			want: 2,
		},
		{
			name: "username collision is retried",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return([]*domain.Token{{ID: "t1", Name: "ci"}}, nil)
				gomock.InOrder(
					m.store.EXPECT().
						SetUsername(gomock.Any(), "t1", gomock.Any()).
						Return(domain.ErrTokenUsernameTaken).
						Times(4),
					m.store.EXPECT().SetUsername(gomock.Any(), "t1", gomock.Any()).Return(nil),
				)
			},
			want: 1,
		},
		{
			name: "username collision on every attempt fails",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return([]*domain.Token{{ID: "t1", Name: "ci"}}, nil)
				m.store.EXPECT().
					SetUsername(gomock.Any(), "t1", gomock.Any()).
					Return(domain.ErrTokenUsernameTaken).
					Times(5)
			},
			wantErr: "set username for token t1: token username already taken",
		},
		{
			name: "list error",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return(nil, errors.New("db error"))
			},
			wantErr: "list tokens: db error",
		},
		{
			name: "set username error stops the run",
			mockFunc: func(m mocks) {
				m.store.EXPECT().ListAll(gomock.Any()).Return([]*domain.Token{
					{ID: "t1", Name: "ci"},
					{ID: "t2", Name: "deploy"},
				}, nil)
				m.store.EXPECT().SetUsername(gomock.Any(), "t1", gomock.Any()).Return(errors.New("db error"))
			},
			wantErr: "set username for token t1: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m := setupService(t)
			tt.mockFunc(m)

			got, err := svc.EnsureUsernames(t.Context())

			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)

				return
			}
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_EnsureUsernames_PartialProgressIsCounted(t *testing.T) {
	t.Parallel()

	svc, m := setupService(t)
	m.store.EXPECT().ListAll(gomock.Any()).Return([]*domain.Token{
		{ID: "t1", Name: "ci"},
		{ID: "t2", Name: "deploy"},
	}, nil)
	m.store.EXPECT().SetUsername(gomock.Any(), "t1", gomock.Any()).Return(nil)
	m.store.EXPECT().SetUsername(gomock.Any(), "t2", gomock.Any()).Return(errors.New("db error"))

	got, err := svc.EnsureUsernames(t.Context())

	require.ErrorContains(t, err, "set username for token t2: db error")
	assert.Equal(t, 1, got)
}
