package config_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/usecase/config"
)

// callLog records transaction boundaries and watch notifications in the order
// they happen, which is the only thing that distinguishes "published after the
// commit" from "published during it". gomock can assert that a call happened and
// with what; it cannot assert when.
type callLog struct {
	mu      sync.Mutex
	entries []string
}

func (l *callLog) add(entry string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, entry)
}

func (l *callLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return slices.Clone(l.entries)
}

// expectLoggedTx makes the transaction manager bracket its callback with begin
// and commit/rollback entries in the log.
//
// The mock cannot flatten a nested WithTx the way the real manager does, so
// PutKey's own boundary shows up as a second begin/commit pair inside the outer
// one. That is visible in the expected logs below and is exactly what the
// pending-events collector has to survive: only the outermost caller flushes.
func expectLoggedTx(m mocks, log *callLog, times int) {
	m.txm.EXPECT().WithTx(gomock.Any(), gomock.Any()).Times(times).DoAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			log.add("tx begin")

			if err := fn(ctx); err != nil {
				log.add("tx rollback")

				return err
			}

			log.add("tx commit")

			return nil
		},
	)
}

// putOp is the transaction operation the tests below write with.
func putOp(namespace, path, value string) config.KVOp {
	return config.KVOp{
		Kind: config.KVOpPut,
		Put: &config.KVPutOp{
			Namespace: namespace,
			Path:      path,
			Value:     []byte(value),
		},
	}
}

// expectPut sets up the full dependency chain one Put inside a transaction
// drives: schema validation, then the repo write.
func expectPut(m mocks, namespace, path, value string, rev int64) {
	m.schemaValidator.EXPECT().
		Validate(gomock.Any(), namespace, path, value, domain.FormatJSON).
		Return(nil)
	m.kv.EXPECT().
		PutKey(gomock.Any(), namespace, path, []byte(value), gomock.Any()).
		Return(nil, rev, nil)
}

func TestService_Txn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    config.KVTxnInput
		mockFunc func(m mocks)
		wantErr  string
		want     config.KVTxnResult
	}{
		{
			name: "every compare holds so the success branch runs",
			input: config.KVTxnInput{
				Compare: []config.KVCompare{{
					StartNS:   "prod",
					StartPath: "/lock.json",
					Target:    config.KVCompareValue,
					Result:    config.KVCompareEqual,
					WantValue: []byte("held"),
				}},
				Success: []config.KVOp{putOp("prod", "/win.json", "yes")},
				Failure: []config.KVOp{putOp("prod", "/lose.json", "no")},
			},
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 2)
				m.kv.EXPECT().
					RangeQuery(
						gomock.Any(), "prod", "/lock.json", "", "",
						int64(0), int64(0), false,
					).
					Return([]*domain.KVPair{{Value: []byte("held")}}, false, nil)
				expectPut(m, "prod", "/win.json", "yes", 12)
			},
			want: config.KVTxnResult{
				Succeeded: true,
				Revision:  12,
				Responses: []config.KVOpResult{{
					Kind: config.KVOpPut,
					Put:  &config.KVPutResult{Revision: 12},
				}},
			},
		},
		{
			name: "one failing compare selects the failure branch",
			input: config.KVTxnInput{
				Compare: []config.KVCompare{
					{
						StartNS:   "prod",
						StartPath: "/lock.json",
						Target:    config.KVCompareValue,
						Result:    config.KVCompareEqual,
						WantValue: []byte("held"),
					},
					{
						StartNS:      "prod",
						StartPath:    "/lock.json",
						Target:       config.KVCompareVersion,
						Result:       config.KVCompareEqual,
						WantRevision: 999,
					},
				},
				Success: []config.KVOp{putOp("prod", "/win.json", "yes")},
				Failure: []config.KVOp{putOp("prod", "/lose.json", "no")},
			},
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 2)
				// Two compares over the same key — the first holds, the second
				// does not, and evaluation stops there.
				m.kv.EXPECT().
					RangeQuery(
						gomock.Any(), "prod", "/lock.json", "", "",
						int64(0), int64(0), false,
					).
					Times(2).
					Return([]*domain.KVPair{{Value: []byte("held"), Version: 1}}, false, nil)
				expectPut(m, "prod", "/lose.json", "no", 13)
			},
			want: config.KVTxnResult{
				Succeeded: false,
				Revision:  13,
				Responses: []config.KVOpResult{{
					Kind: config.KVOpPut,
					Put:  &config.KVPutResult{Revision: 13},
				}},
			},
		},
		{
			// The create-if-absent lock recipe: an empty range makes every
			// revision target compare against 0 and the value target against
			// nil, so Compare(CreateRevision, =, 0) holds.
			name: "absent key makes create-if-absent hold",
			input: config.KVTxnInput{
				Compare: []config.KVCompare{{
					StartNS:      "prod",
					StartPath:    "/lock.json",
					Target:       config.KVCompareCreateRevision,
					Result:       config.KVCompareEqual,
					WantRevision: 0,
				}},
				Success: []config.KVOp{putOp("prod", "/lock.json", "mine")},
			},
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 2)
				m.kv.EXPECT().
					RangeQuery(
						gomock.Any(), "prod", "/lock.json", "", "",
						int64(0), int64(0), false,
					).
					Return(nil, false, nil)
				expectPut(m, "prod", "/lock.json", "mine", 1)
			},
			want: config.KVTxnResult{
				Succeeded: true,
				Revision:  1,
				Responses: []config.KVOpResult{{
					Kind: config.KVOpPut,
					Put:  &config.KVPutResult{Revision: 1},
				}},
			},
		},
		{
			name: "a range op does not advance the revision so the current one is fetched",
			input: config.KVTxnInput{
				Success: []config.KVOp{{
					Kind: config.KVOpRange,
					Range: &config.KVRangeOp{
						StartNS:   "prod",
						StartPath: "/a.json",
						Opts:      config.KVRangeOpts{Limit: 2},
					},
				}},
			},
			mockFunc: func(m mocks) {
				m.kv.EXPECT().
					RangeQuery(
						gomock.Any(), "prod", "/a.json", "", "",
						int64(2), int64(0), false,
					).
					Return([]*domain.KVPair{{Namespace: "prod", Path: "/a.json"}}, true, nil)
				m.kv.EXPECT().CurrentRevisionValue(gomock.Any()).Return(int64(8), nil)
			},
			want: config.KVTxnResult{
				Succeeded: true,
				Revision:  8,
				Responses: []config.KVOpResult{{
					Kind: config.KVOpRange,
					Range: &config.KVRangeResult{
						KVs:      []*domain.KVPair{{Namespace: "prod", Path: "/a.json"}},
						Revision: 8,
						More:     true,
					},
				}},
			},
		},
		{
			name:  "an empty transaction reports the current revision",
			input: config.KVTxnInput{},
			mockFunc: func(m mocks) {
				m.kv.EXPECT().CurrentRevisionValue(gomock.Any()).Return(int64(5), nil)
			},
			want: config.KVTxnResult{
				Succeeded: true,
				Revision:  5,
				Responses: []config.KVOpResult{},
			},
		},
		{
			name: "an unknown op kind is a validation error",
			input: config.KVTxnInput{
				Success: []config.KVOp{{Kind: config.KVOpKind(99)}},
			},
			mockFunc: func(_ mocks) {},
			wantErr:  "run op: validation: op: unknown transaction operation",
		},
		{
			name: "a nested transaction with no body is a validation error",
			input: config.KVTxnInput{
				Success: []config.KVOp{{Kind: config.KVOpTxn}},
			},
			mockFunc: func(_ mocks) {},
			wantErr:  "run op: validation: txn: nested transaction is empty",
		},
		{
			name: "a failing compare read aborts the transaction",
			input: config.KVTxnInput{
				Compare: []config.KVCompare{{
					StartNS:   "prod",
					StartPath: "/lock.json",
					Target:    config.KVCompareValue,
					Result:    config.KVCompareEqual,
				}},
				Success: []config.KVOp{putOp("prod", "/win.json", "yes")},
			},
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 1)
				m.kv.EXPECT().
					RangeQuery(
						gomock.Any(), "prod", "/lock.json", "", "",
						int64(0), int64(0), false,
					).
					Return(nil, false, errors.New("db error"))
			},
			wantErr: "txn tx: compare range: range query: db error",
		},
		{
			name: "a failing op aborts the transaction",
			input: config.KVTxnInput{
				Success: []config.KVOp{putOp("prod", "/win.json", "yes")},
			},
			mockFunc: func(m mocks) {
				expectPassthroughTx(m, 2)
				m.schemaValidator.EXPECT().
					Validate(gomock.Any(), "prod", "/win.json", "yes", domain.FormatJSON).
					Return(nil)
				m.kv.EXPECT().
					PutKey(gomock.Any(), "prod", "/win.json", []byte("yes"), gomock.Any()).
					Return(nil, int64(0), errors.New("db error"))
			},
			wantErr: "txn tx: txn put: put key tx: put key: db error",
		},
		{
			name: "a nested transaction's result is carried in the response",
			input: config.KVTxnInput{
				Success: []config.KVOp{{
					Kind: config.KVOpTxn,
					Txn: &config.KVTxnInput{
						Success: []config.KVOp{putOp("prod", "/nested.json", "v")},
					},
				}},
			},
			mockFunc: func(m mocks) {
				// Outer Txn, nested Txn, and the nested Put each open a
				// boundary the mock cannot flatten.
				expectPassthroughTx(m, 3)
				expectPut(m, "prod", "/nested.json", "v", 4)
			},
			want: config.KVTxnResult{
				Succeeded: true,
				Revision:  4,
				Responses: []config.KVOpResult{{
					Kind: config.KVOpTxn,
					Txn: &config.KVTxnResult{
						Succeeded: true,
						Revision:  4,
						Responses: []config.KVOpResult{{
							Kind: config.KVOpPut,
							Put:  &config.KVPutResult{Revision: 4},
						}},
					},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, m, _ := setupService(t)

			capture := &notifyCapture{}
			captureNotifications(m, capture)
			tt.mockFunc(m)

			got, err := svc.Txn(t.Context(), tt.input)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestService_Txn_PublishesAfterTheCommit(t *testing.T) {
	t.Parallel()

	svc, m, _ := setupService(t)

	log := &callLog{}

	expectLoggedTx(m, log, 2)
	m.schemaValidator.EXPECT().
		Validate(gomock.Any(), "prod", "/lock.json", "mine", domain.FormatJSON).
		Return(nil)
	m.kv.EXPECT().
		RangeQuery(gomock.Any(), "prod", "/lock.json", "", "", int64(0), int64(0), false).
		Return(nil, false, nil)
	m.kv.EXPECT().
		PutKey(gomock.Any(), "prod", "/lock.json", []byte("mine"), gomock.Any()).
		Return(nil, int64(1), nil)
	m.watcher.EXPECT().
		NotifyCreated(gomock.Any(), gomock.Any()).
		Do(func(_ context.Context, _ *domain.Config) { log.add("notify created") })

	_, err := svc.Txn(t.Context(), config.KVTxnInput{
		Compare: []config.KVCompare{{
			StartNS:      "prod",
			StartPath:    "/lock.json",
			Target:       config.KVCompareCreateRevision,
			Result:       config.KVCompareEqual,
			WantRevision: 0,
		}},
		Success: []config.KVOp{putOp("prod", "/lock.json", "mine")},
	})
	require.NoError(t, err)

	// A notification before the outermost "tx commit" would let a watcher
	// observe a write a later failing operation rolls back.
	assert.Equal(t, []string{
		"tx begin",
		"tx begin",
		"tx commit",
		"tx commit",
		"notify created",
	}, log.snapshot())
}

func TestService_Txn_PublishesNothingOnRollback(t *testing.T) {
	t.Parallel()

	svc, m, _ := setupService(t)

	// No watcher expectation at all: gomock fails the test on any call, which
	// is a stronger statement than counting zero afterwards. The first Put
	// succeeds and buffers its event, so the buffer is non-empty when the
	// second one fails — the event still must not reach the watcher.
	expectPassthroughTx(m, 3)
	expectPut(m, "prod", "/a.json", "v", 1)
	m.schemaValidator.EXPECT().
		Validate(gomock.Any(), "prod", "/b.json", "v", domain.FormatJSON).
		Return(nil)
	m.kv.EXPECT().
		PutKey(gomock.Any(), "prod", "/b.json", []byte("v"), gomock.Any()).
		Return(nil, int64(0), errors.New("db error"))

	_, err := svc.Txn(t.Context(), config.KVTxnInput{
		Success: []config.KVOp{
			putOp("prod", "/a.json", "v"),
			putOp("prod", "/b.json", "v"),
		},
	})
	require.ErrorContains(t, err, "txn tx: txn put: put key tx: put key: db error")
}

func TestService_Txn_PublishesEachWriteOnceAfterTheOuterCommit(t *testing.T) {
	t.Parallel()

	svc, m, _ := setupService(t)

	log := &callLog{}

	expectLoggedTx(m, log, 3)
	expectPut(m, "prod", "/a.json", "v", 1)
	expectPut(m, "prod", "/b.json", "v", 2)
	m.watcher.EXPECT().
		NotifyCreated(gomock.Any(), gomock.Any()).
		Times(2).
		Do(func(_ context.Context, cfg *domain.Config) { log.add("notify " + cfg.Path) })

	res, err := svc.Txn(t.Context(), config.KVTxnInput{
		Success: []config.KVOp{
			putOp("prod", "/a.json", "v"),
			putOp("prod", "/b.json", "v"),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), res.Revision, "the response header carries the highest revision")

	// Two writes, two notifications, one flush: both land after the outermost
	// commit rather than one after each inner boundary.
	assert.Equal(t, []string{
		"tx begin",
		"tx begin",
		"tx commit",
		"tx begin",
		"tx commit",
		"tx commit",
		"notify /a.json",
		"notify /b.json",
	}, log.snapshot())
}

func TestService_Txn_ReadOnlyOpensNoTransaction(t *testing.T) {
	t.Parallel()

	svc, m, _ := setupService(t)

	// No WithTx expectation: bbolt permits a single writer, so opening a
	// writable transaction for a transaction that only reads would serialise
	// every read behind every write. gomock fails on the unexpected call.
	m.kv.EXPECT().
		RangeQuery(gomock.Any(), "prod", "/a.json", "", "", int64(0), int64(0), false).
		Return([]*domain.KVPair{{Namespace: "prod", Path: "/a.json"}}, false, nil)
	m.kv.EXPECT().CurrentRevisionValue(gomock.Any()).Return(int64(3), nil)

	res, err := svc.Txn(t.Context(), config.KVTxnInput{
		Success: []config.KVOp{{
			Kind:  config.KVOpRange,
			Range: &config.KVRangeOp{StartNS: "prod", StartPath: "/a.json"},
		}},
		Failure: []config.KVOp{{
			Kind:  config.KVOpRange,
			Range: &config.KVRangeOp{StartNS: "prod", StartPath: "/b.json"},
		}},
	})
	require.NoError(t, err)
	assert.True(t, res.Succeeded)
	assert.Equal(t, int64(3), res.Revision)
}

func TestService_Txn_WriteInTheUntakenBranchStillOpensTransaction(t *testing.T) {
	t.Parallel()

	svc, m, _ := setupService(t)

	log := &callLog{}

	// The branch is not known until the compares have been evaluated, and
	// evaluating them is part of the transaction — so a write that only the
	// failure branch would perform still takes the writable path.
	expectLoggedTx(m, log, 1)
	m.kv.EXPECT().
		RangeQuery(gomock.Any(), "prod", "/a.json", "", "", int64(0), int64(0), false).
		Return([]*domain.KVPair{{Namespace: "prod", Path: "/a.json"}}, false, nil)
	m.kv.EXPECT().CurrentRevisionValue(gomock.Any()).Return(int64(3), nil)

	res, err := svc.Txn(t.Context(), config.KVTxnInput{
		Success: []config.KVOp{{
			Kind:  config.KVOpRange,
			Range: &config.KVRangeOp{StartNS: "prod", StartPath: "/a.json"},
		}},
		Failure: []config.KVOp{putOp("prod", "/b.json", "v")},
	})
	require.NoError(t, err)
	assert.True(t, res.Succeeded)
	assert.Equal(t, []string{"tx begin", "tx commit"}, log.snapshot())
}
