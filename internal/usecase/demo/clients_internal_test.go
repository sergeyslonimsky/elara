package demo

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

// recordedWatch and recordedRequest capture the arguments seedClients passes to
// the registry so the test can assert on the exact data flow.
type recordedWatch struct {
	connID string
	watch  domain.ActiveWatch
}

type recordedRequest struct {
	connID   string
	method   string
	key      string
	revision int64
	duration time.Duration
	err      error
}

// fakeRegistry is a hand-written capturing double for the package-private
// clientRegistry interface. gomock is not used here because clientRegistry is
// unexported and has no generated mock; a capturing fake keeps the per-call
// assertions (counts + argument values) readable.
type fakeRegistry struct {
	nextID      int
	connections []domain.ConnectionInfo
	watches     []recordedWatch
	requests    []recordedRequest
}

func (f *fakeRegistry) RegisterConnection(info domain.ConnectionInfo) string {
	f.nextID++
	id := fmt.Sprintf("conn-%d", f.nextID)
	f.connections = append(f.connections, info)

	return id
}

func (f *fakeRegistry) RegisterWatch(connID string, w domain.ActiveWatch) {
	f.watches = append(f.watches, recordedWatch{connID: connID, watch: w})
}

func (f *fakeRegistry) RecordRequest(
	connID, method, key string,
	revision int64,
	duration time.Duration,
	err error,
) {
	f.requests = append(f.requests, recordedRequest{
		connID:   connID,
		method:   method,
		key:      key,
		revision: revision,
		duration: duration,
		err:      err,
	})
}

func TestSeedClients(t *testing.T) {
	t.Parallel()

	reg := &fakeRegistry{}

	seedClients(reg)

	// One RegisterConnection and one RegisterWatch per sample client.
	require.Len(t, reg.connections, len(sampleClients))
	require.Len(t, reg.watches, len(sampleClients))

	// One RecordRequest per rangeCall plus one final Watch call per client.
	wantRequests := len(sampleClients)
	for _, c := range sampleClients {
		wantRequests += c.rangeCalls
	}
	require.Len(t, reg.requests, wantRequests)

	// Requests grouped by the connection id the fake handed back.
	byConn := make(map[string][]recordedRequest)
	for _, r := range reg.requests {
		byConn[r.connID] = append(byConn[r.connID], r)
	}

	for i, c := range sampleClients {
		// RegisterConnection receives the exact ConnectionInfo from the table.
		assert.Equal(t, c.info, reg.connections[i], "connection %d info", i)

		// Connection ids are handed out in order, starting at conn-1.
		connID := fmt.Sprintf("conn-%d", i+1)

		w := reg.watches[i]
		assert.Equal(t, connID, w.connID, "watch %d connID", i)
		assert.Equal(t, int64(i+1), w.watch.WatchID, "watch %d id", i)
		assert.Equal(t, c.watchKey, w.watch.StartKey, "watch %d key", i)
		assert.Equal(t, c.watchRev, w.watch.StartRevision, "watch %d revision", i)
		assert.True(t, w.watch.ProgressNotify, "watch %d progress notify", i)

		reqs := byConn[connID]
		require.Len(t, reqs, c.rangeCalls+1, "requests for %s", connID)

		var ranges, watches int
		for _, r := range reqs {
			assert.Equal(t, c.watchKey, r.key, "%s request key", connID)
			assert.Equal(t, c.watchRev, r.revision, "%s request revision", connID)
			require.NoErrorf(t, r.err, "%s request err", connID)

			switch r.method {
			case methodRange:
				ranges++
			case methodWatch:
				watches++
			default:
				t.Fatalf("unexpected method %q for %s", r.method, connID)
			}
		}

		assert.Equal(t, c.rangeCalls, ranges, "%s Range calls", connID)
		assert.Equal(t, 1, watches, "%s Watch calls", connID)
	}
}
