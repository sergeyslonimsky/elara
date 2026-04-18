package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
)

const (
	defaultHistoryBuffer    = 256
	defaultHistoryRetainN   = 1000
	defaultHistoryRetainAge = 30 * 24 * time.Hour
)

// ClientHistoryRepo is what HistoryStore needs from a persistence layer.
// The bbolt adapter implements this.
type ClientHistoryRepo interface {
	Save(ctx context.Context, c *domain.Client) error
	List(ctx context.Context, limit int) ([]*domain.Client, error)
	ListByClient(ctx context.Context, clientName, k8sNamespace string, limit int) ([]*domain.Client, error)
	Count(ctx context.Context) (int, error)
	DeleteOldest(ctx context.Context, n int) (int, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error)
}

// HistoryConfig controls retention and async behaviour.
type HistoryConfig struct {
	// MaxRecords is the cap on retained snapshots. Older are evicted on write.
	// Zero or negative → default (1000).
	MaxRecords int
	// MaxAge bounds the age of retained snapshots. Older are evicted on write.
	// Zero → default (30 days). Negative → disabled (no age-based eviction).
	MaxAge time.Duration
	// BufferSize is the size of the async write channel. Zero → default (256).
	BufferSize int
}

func (c HistoryConfig) withDefaults() HistoryConfig {
	if c.MaxRecords <= 0 {
		c.MaxRecords = defaultHistoryRetainN
	}

	if c.MaxAge == 0 {
		c.MaxAge = defaultHistoryRetainAge
	}

	if c.BufferSize <= 0 {
		c.BufferSize = defaultHistoryBuffer
	}

	return c
}

// HistoryStore wraps a ClientHistoryRepo with an async write goroutine and
// retention enforcement. It implements HistorySink so Registry can use it
// directly.
//
// Record() is non-blocking: if the buffer is full, the oldest pending write
// is dropped (with a warn log) to keep the hot disconnect path fast.
type HistoryStore struct {
	cfg  HistoryConfig
	repo ClientHistoryRepo

	in chan *domain.Client

	// mu protects shutdown. Record() holds mu across the channel send so that
	// Shutdown() cannot close s.in between the "are we shut down?" check and
	// the actual send — which would cause a panic.
	mu       sync.Mutex
	shutdown bool

	stopOnce sync.Once
	done     chan struct{}
}

func NewHistoryStore(ctx context.Context, cfg HistoryConfig, repo ClientHistoryRepo) *HistoryStore {
	cfg = cfg.withDefaults()

	s := &HistoryStore{
		cfg:  cfg,
		repo: repo,
		in:   make(chan *domain.Client, cfg.BufferSize),
		done: make(chan struct{}),
	}

	go s.run(ctx)

	return s
}

// Record queues a snapshot for async persistence. Non-blocking; drops on full
// buffer with a warn log.
func (s *HistoryStore) Record(c *domain.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.shutdown {
		return
	}

	select {
	case s.in <- c:
	default:
		slog.Warn("monitor.history: snapshot dropped: write buffer full",
			"client_id", c.ID,
		)
	}
}

// List forwards to the repo. Returned snapshots are newest-first.
func (s *HistoryStore) List(ctx context.Context, limit int) ([]*domain.Client, error) {
	out, err := s.repo.List(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list client history: %w", err)
	}

	return out, nil
}

// ListByClient forwards to the repo. Returned snapshots are newest-first.
func (s *HistoryStore) ListByClient(
	ctx context.Context,
	clientName, k8sNamespace string,
	limit int,
) ([]*domain.Client, error) {
	out, err := s.repo.ListByClient(ctx, clientName, k8sNamespace, limit)
	if err != nil {
		return nil, fmt.Errorf("list client history by client: %w", err)
	}

	return out, nil
}

// Shutdown signals the writer to exit, drains the in-flight queue, and waits
// for the background goroutine to finish.
func (s *HistoryStore) Shutdown() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.shutdown = true
		close(s.in)
		s.mu.Unlock()
	})
	<-s.done
}

// run is the single writer goroutine.
func (s *HistoryStore) run(ctx context.Context) {
	defer close(s.done)

	for snap := range s.in {
		s.persist(ctx, snap)
		s.applyRetention(ctx)
	}
}

func (s *HistoryStore) persist(ctx context.Context, snap *domain.Client) {
	if err := s.repo.Save(ctx, snap); err != nil {
		slog.Warn("monitor.history: save failed",
			"client_id", snap.ID,
			"error", err,
		)
	}
}

func (s *HistoryStore) applyRetention(ctx context.Context) {
	// Age-based first (often deletes more), then count-based.
	if s.cfg.MaxAge > 0 {
		cutoff := time.Now().Add(-s.cfg.MaxAge)

		if _, err := s.repo.DeleteOlderThan(ctx, cutoff); err != nil {
			slog.Warn("monitor.history: age retention failed",
				"cutoff", cutoff,
				"error", err,
			)
		}
	}

	count, err := s.repo.Count(ctx)
	if err != nil {
		slog.Warn("monitor.history: count failed", "error", err)

		return
	}

	if count <= s.cfg.MaxRecords {
		return
	}

	excess := count - s.cfg.MaxRecords
	if _, err := s.repo.DeleteOldest(ctx, excess); err != nil {
		slog.Warn("monitor.history: count retention failed",
			"excess", excess,
			"error", err,
		)
	}
}
