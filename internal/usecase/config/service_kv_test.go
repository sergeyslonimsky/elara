package config_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// notifyCapture records what the service published.
//
// The watch payload is not a return value, so it cannot be asserted from the
// call's result; recording it and checking afterwards keeps the assertion in the
// test body rather than inside a mock expectation.
type notifyCapture struct {
	mu      sync.Mutex
	created []*domain.Config
	updated []*domain.Config
	deleted []deletedEvent
}

type deletedEvent struct {
	path      string
	namespace string
	revision  int64
}

// captureNotifications installs recording expectations for the three KV-path
// notifications. AnyTimes because "published nothing" is a legitimate outcome
// the test body asserts on the capture, not on the expectation.
func captureNotifications(m mocks, c *notifyCapture) {
	m.watcher.EXPECT().
		NotifyCreated(gomock.Any(), gomock.Any()).
		AnyTimes().
		Do(func(_ context.Context, cfg *domain.Config) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.created = append(c.created, cfg)
		})

	m.watcher.EXPECT().
		NotifyUpdated(gomock.Any(), gomock.Any()).
		AnyTimes().
		Do(func(_ context.Context, cfg *domain.Config) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.updated = append(c.updated, cfg)
		})

	m.watcher.EXPECT().
		NotifyDeleted(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().
		Do(func(_ context.Context, path, namespace string, revision int64) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.deleted = append(c.deleted, deletedEvent{path, namespace, revision})
		})
}

func (c *notifyCapture) snapshot() ([]*domain.Config, []*domain.Config, []deletedEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.created, c.updated, c.deleted
}

// stripTimestamps blanks the wall-clock fields the published config carries, so
// the rest can be compared as a whole value.
func stripTimestamps(cfgs []*domain.Config) []*domain.Config {
	if len(cfgs) == 0 {
		return nil
	}

	out := make([]*domain.Config, 0, len(cfgs))

	for _, cfg := range cfgs {
		clone := *cfg
		clone.CreatedAt = time.Time{}
		clone.UpdatedAt = time.Time{}
		out = append(out, &clone)
	}

	return out
}

// expectPassthroughTx makes the transaction manager run its callback with the
// context it was handed. The real manager flattens a nested WithTx onto the
// outer transaction; the mock cannot, so a nested call shows up as another
// invocation here — times says how many the flow under test makes.
func expectPassthroughTx(m mocks, times int) {
	m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).Times(times).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		},
	)
}

func TestService_PutKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		namespace   string
		path        string
		value       []byte
		mockFunc    func(m mocks)
		wantErr     string
		wantPrev    *domain.KVPair
		wantRev     int64
		wantCreated []*domain.Config
		wantUpdated []*domain.Config
	}{
		{
			name:      "no previous value publishes created",
			namespace: "prod",
			path:      "/lock.json",
			value:     []byte(`{"a":1}`),
			mockFunc: func(m mocks) {
				m.schemaValidator.EXPECT().
					Validate(gomock.Any(), "prod", "/lock.json", `{"a":1}`, domain.FormatJSON).
					Return(nil)
				expectPassthroughTx(m, 1)
				m.kv.EXPECT().
					PutKey(gomock.Any(), "prod", "/lock.json", []byte(`{"a":1}`)).
					Return(nil, int64(7), nil)
			},
			wantRev: 7,
			wantCreated: []*domain.Config{{
				Path:           "/lock.json",
				Namespace:      "prod",
				Content:        `{"a":1}`,
				Format:         domain.FormatJSON,
				Revision:       7,
				Version:        1,
				CreateRevision: 7,
			}},
		},
		{
			name:      "previous value publishes updated and preserves create revision",
			namespace: "prod",
			path:      "/lock.json",
			value:     []byte(`{"a":2}`),
			mockFunc: func(m mocks) {
				m.schemaValidator.EXPECT().
					Validate(gomock.Any(), "prod", "/lock.json", `{"a":2}`, domain.FormatJSON).
					Return(nil)
				expectPassthroughTx(m, 1)
				m.kv.EXPECT().
					PutKey(gomock.Any(), "prod", "/lock.json", []byte(`{"a":2}`)).
					Return(&domain.KVPair{Version: 4, CreateRevision: 2}, int64(9), nil)
			},
			wantPrev: &domain.KVPair{Version: 4, CreateRevision: 2},
			wantRev:  9,
			wantUpdated: []*domain.Config{{
				Path:           "/lock.json",
				Namespace:      "prod",
				Content:        `{"a":2}`,
				Format:         domain.FormatJSON,
				Revision:       9,
				Version:        5,
				CreateRevision: 2,
			}},
		},
		{
			name:      "schema validation rejects before any transaction opens",
			namespace: "prod",
			path:      "/lock.json",
			value:     []byte(`{"a":1}`),
			mockFunc: func(m mocks) {
				m.schemaValidator.EXPECT().
					Validate(gomock.Any(), "prod", "/lock.json", `{"a":1}`, domain.FormatJSON).
					Return(errors.New("schema mismatch"))
			},
			wantErr: "schema validation: schema mismatch",
		},
		{
			name:      "storage error publishes nothing",
			namespace: "prod",
			path:      "/lock.json",
			value:     []byte(`{"a":1}`),
			mockFunc: func(m mocks) {
				m.schemaValidator.EXPECT().
					Validate(gomock.Any(), "prod", "/lock.json", `{"a":1}`, domain.FormatJSON).
					Return(nil)
				expectPassthroughTx(m, 1)
				m.kv.EXPECT().
					PutKey(gomock.Any(), "prod", "/lock.json", []byte(`{"a":1}`)).
					Return(nil, int64(0), errors.New("db error"))
			},
			wantErr: "put key tx: put key: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			capture := &notifyCapture{}
			captureNotifications(m, capture)
			tt.mockFunc(m)

			prev, rev, err := svc.PutKey(t.Context(), tt.namespace, tt.path, tt.value)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				created, updated, deleted := capture.snapshot()
				assert.Empty(t, created)
				assert.Empty(t, updated)
				assert.Empty(t, deleted)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPrev, prev)
			assert.Equal(t, tt.wantRev, rev)

			created, updated, deleted := capture.snapshot()
			assert.Equal(t, tt.wantCreated, stripTimestamps(created))
			assert.Equal(t, tt.wantUpdated, stripTimestamps(updated))
			assert.Empty(t, deleted)
		})
	}
}

func TestService_PutKey_CreatedCarriesCreationTimestamp(t *testing.T) {
	t.Parallel()

	svc, m, _ := setupService(t)

	capture := &notifyCapture{}
	captureNotifications(m, capture)

	m.schemaValidator.EXPECT().
		Validate(gomock.Any(), "prod", "/a.json", "v", domain.FormatJSON).
		Return(nil)
	expectPassthroughTx(m, 1)
	m.kv.EXPECT().
		PutKey(gomock.Any(), "prod", "/a.json", []byte("v")).
		Return(nil, int64(1), nil)

	_, _, err := svc.PutKey(t.Context(), "prod", "/a.json", []byte("v"))
	require.NoError(t, err)

	created, _, _ := capture.snapshot()
	require.Len(t, created, 1)
	assert.False(t, created[0].CreatedAt.IsZero(), "a created config carries CreatedAt")
	assert.Equal(t, created[0].UpdatedAt, created[0].CreatedAt)
}

func TestService_DeleteRangeKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mockFunc    func(m mocks)
		wantErr     string
		wantDeleted []*domain.KVPair
		wantRev     int64
		wantEvents  []deletedEvent
	}{
		{
			name: "one notification per deleted pair",
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 1)
				m.kv.EXPECT().
					DeleteRangeKeys(gomock.Any(), "prod", "/", "prod0", "/", true).
					Return([]*domain.KVPair{
						{Namespace: "prod", Path: "/a.json"},
						{Namespace: "prod", Path: "/b.json"},
					}, int64(11), nil)
			},
			wantDeleted: []*domain.KVPair{
				{Namespace: "prod", Path: "/a.json"},
				{Namespace: "prod", Path: "/b.json"},
			},
			wantRev: 11,
			wantEvents: []deletedEvent{
				{path: "/a.json", namespace: "prod", revision: 11},
				{path: "/b.json", namespace: "prod", revision: 11},
			},
		},
		{
			name: "nothing matched publishes nothing",
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 1)
				m.kv.EXPECT().
					DeleteRangeKeys(gomock.Any(), "prod", "/", "prod0", "/", true).
					Return(nil, int64(0), nil)
			},
			wantRev: 0,
		},
		{
			name: "storage error publishes nothing",
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 1)
				m.kv.EXPECT().
					DeleteRangeKeys(gomock.Any(), "prod", "/", "prod0", "/", true).
					Return(nil, int64(0), errors.New("db error"))
			},
			wantErr: "delete range keys tx: delete range keys: db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			capture := &notifyCapture{}
			captureNotifications(m, capture)
			tt.mockFunc(m)

			deleted, rev, err := svc.DeleteRangeKeys(
				t.Context(),
				"prod", "/", "prod0", "/",
				true,
			)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				_, _, events := capture.snapshot()
				assert.Empty(t, events)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantDeleted, deleted)
			assert.Equal(t, tt.wantRev, rev)

			created, updated, events := capture.snapshot()
			assert.Empty(t, created)
			assert.Empty(t, updated)
			assert.Equal(t, tt.wantEvents, nilIfEmpty(events))
		})
	}
}

// nilIfEmpty normalises an unset capture to nil so a case that expects no
// notifications can leave its want field out.
func nilIfEmpty(events []deletedEvent) []deletedEvent {
	if len(events) == 0 {
		return nil
	}

	return events
}
