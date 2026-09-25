package watch_test

import (
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/sergeyslonimsky/elara/internal/domain"
	"github.com/sergeyslonimsky/elara/internal/transport/watch"
)

// Benchmarks for watch fan-out: the cost of turning one mutation into N
// deliveries.
//
// notify() holds a read lock over the whole subscription map and walks every
// entry, so the per-event cost grows with the number of connected watchers
// regardless of how many of them actually match. The three shapes below
// separate that walk from the delivery itself.

func BenchmarkPublisher_FanOut_Matching(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()

			pub := watch.NewPublisher()
			chans := subscribeN(b, pub, n, "/svc/", "ns")
			cfg := benchConfig()
			ctx := b.Context()

			b.ResetTimer()

			for b.Loop() {
				pub.NotifyUpdated(ctx, cfg)

				// Draining is part of the measurement, not an artifact of it:
				// the subscription buffer is bounded at 100 and a full buffer
				// drops instead of blocking, so an undrained loop would stop
				// measuring delivery after 100 events and start measuring the
				// drop path (see the dropped-buffer benchmark below).
				for _, ch := range chans {
					<-ch
				}
			}
		})
	}
}

// BenchmarkPublisher_FanOut_NonMatching isolates the map walk and prefix
// filter from delivery: every subscriber is registered on a prefix the event
// does not match, so notify() iterates the full map and delivers nothing.
// The delta against the matching benchmark at the same N is the cost of
// actually handing the event over.
func BenchmarkPublisher_FanOut_NonMatching(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()

			pub := watch.NewPublisher()
			subscribeN(b, pub, n, "/other/", "ns")
			cfg := benchConfig()
			ctx := b.Context()

			b.ResetTimer()

			for b.Loop() {
				pub.NotifyUpdated(ctx, cfg)
			}
		})
	}
}

// BenchmarkPublisher_FanOut_DroppedFullBuffer measures the slow-subscriber
// path: nothing is drained, so after the first 100 events every delivery
// falls through to the drop branch. This is a real production shape — a
// watcher that stops reading does not slow the writer down, it loses events —
// and it is worth knowing what the publisher pays for each loss.
func BenchmarkPublisher_FanOut_DroppedFullBuffer(b *testing.B) {
	b.ReportAllocs()

	// The drop branch logs a warning per dropped event. Left on the default
	// handler that write would dominate the measurement and flood the
	// benchmark output, so the logger is muted for this benchmark only.
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.DiscardHandler))

	b.Cleanup(func() { slog.SetDefault(prev) })

	pub := watch.NewPublisher()
	subscribeN(b, pub, 100, "/svc/", "ns")
	cfg := benchConfig()
	ctx := b.Context()

	b.ResetTimer()

	for b.Loop() {
		pub.NotifyUpdated(ctx, cfg)
	}
}

// BenchmarkPublisher_SubscribeUnsubscribe covers watcher churn rather than
// event throughput: both ends take the write lock, so a reconnect storm
// contends with every in-flight notify.
func BenchmarkPublisher_SubscribeUnsubscribe(b *testing.B) {
	b.ReportAllocs()

	pub := watch.NewPublisher()
	ctx := b.Context()

	b.ResetTimer()

	for b.Loop() {
		_, cleanup := pub.Subscribe(ctx, "/svc/", "ns")
		cleanup()
	}
}

// subscribeN registers n subscribers on the same prefix and namespace and
// returns their channels. Cleanup is registered with the benchmark so a
// failed run does not leak subscriptions into the next one.
func subscribeN(
	b *testing.B,
	pub *watch.Publisher,
	n int,
	pathPrefix, namespace string,
) []<-chan domain.WatchEvent {
	b.Helper()

	chans := make([]<-chan domain.WatchEvent, 0, n)

	for range n {
		ch, cleanup := pub.Subscribe(b.Context(), pathPrefix, namespace)
		b.Cleanup(cleanup)
		chans = append(chans, ch)
	}

	return chans
}

// benchConfig builds the event payload once, outside any timed section.
func benchConfig() *domain.Config {
	return &domain.Config{
		Path:      "/svc/app.json",
		Namespace: "ns",
		Content:   `{"replicas":3,"image":"app:1.2.3"}`,
		Format:    domain.FormatJSON,
		Revision:  42,
		Version:   7,
		UpdatedAt: time.Now(),
	}
}
