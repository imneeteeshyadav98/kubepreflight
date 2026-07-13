package k8s

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These are internal (package k8s, not k8s_test) tests for runBoundedPool
// specifically, for the same reason collector_timeout_test.go tests
// collectResource directly rather than through Collect against a fake
// clientset: k8s.io/client-go's generated fake clients never thread ctx
// into their reactor chain, and reactors don't even receive a context
// argument at all, so there is no way to simulate a hung or
// slot-contending call through the fake clientset. runBoundedPool itself
// has no Kubernetes dependency, so it's tested directly with synthetic
// tasks instead.

func TestRunBoundedPool_NeverExceedsConfiguredLimit(t *testing.T) {
	const concurrency = 3
	const taskCount = 30

	var current int32
	var maxObserved int32
	tasks := make([]func(), taskCount)
	for i := range tasks {
		tasks[i] = func() {
			n := atomic.AddInt32(&current, 1)
			for {
				observed := atomic.LoadInt32(&maxObserved)
				if n <= observed || atomic.CompareAndSwapInt32(&maxObserved, observed, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&current, -1)
		}
	}

	runBoundedPool(concurrency, tasks)

	if maxObserved > concurrency {
		t.Errorf("max concurrent tasks = %d, want <= %d", maxObserved, concurrency)
	}
	if maxObserved < concurrency {
		t.Errorf("max concurrent tasks = %d, want exactly %d to prove the pool actually uses all its slots (not a false pass from under-scheduling)", maxObserved, concurrency)
	}
}

func TestRunBoundedPool_ConcurrencyOneFullySerializes(t *testing.T) {
	// concurrency=1 must guarantee no two tasks ever run at once (fully
	// sequential execution -- the property Collect actually relies on for
	// its "concurrency=1 behaves like the original sequential code" claim).
	// It does NOT guarantee tasks run in submission order: all task
	// goroutines are started immediately and race to acquire the sole
	// semaphore slot, and Go's runtime makes no FIFO guarantee about which
	// blocked sender on a buffered channel wins -- confirmed by an earlier
	// version of this test asserting strict order and failing. Collect
	// doesn't depend on resource-kind processing order either: each
	// resource kind writes a distinct Snapshot field, and the one
	// order-sensitive case (DeprecatedAPIUsage, appended into by multiple
	// tasks) is covered separately by
	// TestCollector_Collect_DeterministicAcrossConcurrencyLevels, which
	// sorts before comparing.
	const taskCount = 20
	var completed int32
	var mu sync.Mutex
	var seen = make(map[int]bool)
	var overlapDetected bool
	var inFlight int32

	tasks := make([]func(), taskCount)
	for i := range tasks {
		i := i
		tasks[i] = func() {
			if atomic.AddInt32(&inFlight, 1) > 1 {
				overlapDetected = true
			}
			mu.Lock()
			seen[i] = true
			mu.Unlock()
			time.Sleep(time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			atomic.AddInt32(&completed, 1)
		}
	}

	runBoundedPool(1, tasks)

	if overlapDetected {
		t.Error("concurrency=1 allowed two tasks to run at once, want fully sequential execution")
	}
	if int(completed) != taskCount {
		t.Fatalf("got %d completed tasks, want %d", completed, taskCount)
	}
	if len(seen) != taskCount {
		t.Fatalf("only %d distinct tasks ran, want all %d to have run exactly once", len(seen), taskCount)
	}
}

func TestRunBoundedPool_QueuedTaskUnblocksPromptlyOnCancellation(t *testing.T) {
	// concurrency=1: the first task occupies the sole slot and hangs on its
	// own per-call timeout context; the second task never even starts
	// running its body, it's still waiting on the semaphore. Cancelling the
	// shared parent ctx must unblock BOTH promptly -- the running one
	// because its own context.WithTimeout child is derived from ctx (via
	// collectResource, exercised directly here), and the queued one because
	// slot 1 is released as soon as the first task's call fails fast on
	// cancellation.
	ctx, cancel := context.WithCancel(context.Background())
	c := &Collector{}
	snap := &Snapshot{Errors: map[string]error{}}
	var mu sync.Mutex

	// Both tasks are symmetric on purpose: at concurrency=1, exactly one of
	// them wins the race for the sole semaphore slot and starts running --
	// which one is unspecified (goroutine scheduling gives no submission-
	// order guarantee, the same lesson TestRunBoundedPool_ConcurrencyOneFullySerializes
	// learned the hard way). signalStarted fires from whichever one
	// actually runs, so the test doesn't need to know or care which.
	var startOnce sync.Once
	started := make(chan struct{})
	signalStarted := func() { startOnce.Do(func() { close(started) }) }

	makeTask := func(key string) func() {
		return func() {
			c.collectResource(ctx, time.Hour, &mu, snap, key, func(callCtx context.Context) error {
				signalStarted()
				<-callCtx.Done()
				return callCtx.Err()
			})
		}
	}
	tasks := []func(){makeTask("first"), makeTask("second")}

	done := make(chan struct{})
	go func() {
		runBoundedPool(1, tasks)
		close(done)
	}()

	<-started // one task is now definitely running and holding the sole slot; the other is still queued
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runBoundedPool did not finish within 2s of cancellation -- a queued task did not unblock promptly")
	}

	for _, key := range []string{"first", "second"} {
		if err, ok := snap.Errors[key]; !ok || err == nil {
			t.Errorf("Errors[%q] not set to a cancellation error, want it recorded for both the running and the queued task", key)
		}
	}
}

func TestRunBoundedPool_NoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		tasks := make([]func(), 10)
		for j := range tasks {
			tasks[j] = func() { time.Sleep(time.Millisecond) }
		}
		runBoundedPool(4, tasks)
	}

	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Errorf("goroutine count grew from %d to %d after 200 pooled tasks, want it to stay flat", before, after)
	}
}
