package casbin_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/auth/casbin"
	casbin_mock "github.com/sergeyslonimsky/elara/internal/auth/casbin/mocks"
)

func TestContextPolicyAdapter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupMock func(repo *casbin_mock.MockContextPolicyRepo)
		run       func(t *testing.T, loader casbin.PolicyLoader)
	}{
		{
			name: "Load delegates to repo and returns rules",
			setupMock: func(repo *casbin_mock.MockContextPolicyRepo) {
				rules := [][]string{
					{"p", "role:admin", "*", "*", "*"},
					{"g", "alice", "role:admin", "*"},
				}
				repo.EXPECT().Load(gomock.Any()).Return(rules, nil)
			},
			run: func(t *testing.T, loader casbin.PolicyLoader) {
				t.Helper()

				got, err := loader.Load()
				require.NoError(t, err)
				assert.Len(t, got, 2)
				assert.Equal(t, "p", got[0][0])
				assert.Equal(t, "g", got[1][0])
			},
		},
		{
			name: "Load wraps repo error",
			setupMock: func(repo *casbin_mock.MockContextPolicyRepo) {
				repo.EXPECT().Load(gomock.Any()).Return(nil, errors.New("db unavailable"))
			},
			run: func(t *testing.T, loader casbin.PolicyLoader) {
				t.Helper()

				_, err := loader.Load()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "load policy")
				assert.Contains(t, err.Error(), "db unavailable")
			},
		},
		{
			name: "Save delegates to repo with correct rules",
			setupMock: func(repo *casbin_mock.MockContextPolicyRepo) {
				expectedRules := [][]string{
					{"p", "role:viewer", "*", "config", "read"},
				}
				repo.EXPECT().Save(gomock.Any(), expectedRules).Return(nil)
			},
			run: func(t *testing.T, loader casbin.PolicyLoader) {
				t.Helper()

				rules := [][]string{
					{"p", "role:viewer", "*", "config", "read"},
				}
				err := loader.Save(rules)
				require.NoError(t, err)
			},
		},
		{
			name: "Save wraps repo error",
			setupMock: func(repo *casbin_mock.MockContextPolicyRepo) {
				repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("write failed"))
			},
			run: func(t *testing.T, loader casbin.PolicyLoader) {
				t.Helper()

				err := loader.Save([][]string{{"p", "role:admin", "*", "*", "*"}})
				require.Error(t, err)
				assert.Contains(t, err.Error(), "save policy")
				assert.Contains(t, err.Error(), "write failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := casbin_mock.NewMockContextPolicyRepo(ctrl)
			tt.setupMock(repo)

			loader := casbin.NewContextPolicyLoader(repo, t.Context())
			tt.run(t, loader)
		})
	}
}

func TestNewContextPolicyLoader_ReturnsWorkingAdapter(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := casbin_mock.NewMockContextPolicyRepo(ctrl)

	rules := [][]string{{"p", "role:editor", "*", "config", "write"}}
	repo.EXPECT().Load(gomock.Any()).Return(rules, nil)

	loader := casbin.NewContextPolicyLoader(repo, t.Context())
	require.NotNil(t, loader)

	got, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, rules, got)
}
