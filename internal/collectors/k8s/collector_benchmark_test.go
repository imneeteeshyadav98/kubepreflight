package k8s

import (
	"strconv"
	"testing"
	"time"
)

// BenchmarkRunBoundedPool measures wall-clock time for a fixed batch of
// artificially-latent tasks at each concurrency level -- the honest way to
// benchmark "does bounded concurrency actually overlap work" in this
// codebase.
//
// A Collect()-level benchmark against k8s.io/client-go's fake clientset was
// tried first and rejected: k8s.io/client-go/testing.Fake.Invokes (which
// every fake List/Get call goes through) holds a single mutex for the
// entire reactor chain invocation, including any artificial time.Sleep
// added via a reactor to simulate network latency -- confirmed by reading
// k8s.io/client-go@v0.31.3/testing/fake.go directly. That serializes every
// simulated "call" onto one goroutine at a time regardless of
// --collector-concurrency, so a benchmark built that way would show zero
// improvement from concurrency -- a fake-clientset artifact, not a
// reflection of Collect's real behavior against a real cluster (whose REST
// client has no such global lock; concurrent HTTP requests are genuinely
// independent). runBoundedPool itself has no client-go dependency at all
// (see its doc comment in collector.go), so benchmarking it directly with
// synthetic latency isolates the one thing this PR actually changes --
// whether bounded concurrency overlaps otherwise-serial work -- from that
// unrelated fake-clientset limitation.
//
// Real end-to-end confirmation that this translates into an actual scan
// speedup lives in the PR description / commit message as a manual
// before/after timing against a real cluster (kp-smoke), not as a
// committed benchmark -- a real cluster's absolute round-trip latency
// varies by environment in a way a committed benchmark number would not
// meaningfully represent.
//
//	go test ./internal/collectors/k8s/... -run '^$' -bench BenchmarkRunBoundedPool -benchtime 5x
func BenchmarkRunBoundedPool(b *testing.B) {
	const taskCount = 50
	const simulatedCallLatency = 8 * time.Millisecond

	for _, concurrency := range []int{1, 2, 4, 8, 16} {
		b.Run(concurrencyLabel(concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tasks := make([]func(), taskCount)
				for j := range tasks {
					tasks[j] = func() { time.Sleep(simulatedCallLatency) }
				}
				runBoundedPool(concurrency, tasks)
			}
		})
	}
}

func concurrencyLabel(n int) string {
	if n == 1 {
		return "concurrency=1(sequential-baseline)"
	}
	return "concurrency=" + strconv.Itoa(n)
}
