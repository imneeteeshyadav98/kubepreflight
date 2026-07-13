package k8s

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

// These are internal (package k8s, not k8s_test) tests for collectResource
// specifically, rather than exercising timeout behavior through Collect
// against a fake clientset: k8s.io/client-go's generated fake clients never
// thread ctx into their reactor chain (confirmed against client-go v0.31.3
// -- FakeNodes.List and friends accept ctx but never pass it to
// Fake.Invokes), so a fake-clientset reactor has no way to observe or react
// to context cancellation/deadlines at all. Real clientsets built from a
// rest.Config do respect context via their underlying http.Client, which is
// a client-go/net-http contract this package doesn't need to re-verify.
// What this package IS responsible for is correctly bounding each call and
// recording the result -- exactly what collectResource does, so that's
// what's tested directly here.

func TestCollectResource_TimeoutRecordsError(t *testing.T) {
	snap := &Snapshot{Errors: map[string]error{}}
	c := &Collector{}

	start := time.Now()
	c.collectResource(context.Background(), 20*time.Millisecond, &sync.Mutex{}, snap, "slow-resource", func(callCtx context.Context) error {
		<-callCtx.Done()
		return callCtx.Err()
	})
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("collectResource took %s, want it bounded near the 20ms timeout", elapsed)
	}
	err, ok := snap.Errors["slow-resource"]
	if !ok {
		t.Fatal("Errors[\"slow-resource\"] not set, want the timeout recorded")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Errors[\"slow-resource\"] = %v, want context.DeadlineExceeded", err)
	}
}

func TestCollectResource_CancelledParentContextRecordsError(t *testing.T) {
	snap := &Snapshot{Errors: map[string]error{}}
	c := &Collector{}

	parentCtx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call even starts

	c.collectResource(parentCtx, time.Minute, &sync.Mutex{}, snap, "cancelled-resource", func(callCtx context.Context) error {
		<-callCtx.Done()
		return callCtx.Err()
	})

	err, ok := snap.Errors["cancelled-resource"]
	if !ok {
		t.Fatal("Errors[\"cancelled-resource\"] not set, want the cancellation recorded")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Errors[\"cancelled-resource\"] = %v, want context.Canceled", err)
	}
}

func TestCollectResource_SuccessDoesNotRecordError(t *testing.T) {
	snap := &Snapshot{Errors: map[string]error{}}
	c := &Collector{}

	called := false
	c.collectResource(context.Background(), time.Second, &sync.Mutex{}, snap, "fast-resource", func(callCtx context.Context) error {
		called = true
		return nil
	})

	if !called {
		t.Fatal("fn was never called")
	}
	if err, ok := snap.Errors["fast-resource"]; ok {
		t.Errorf("Errors[\"fast-resource\"] = %v, want no entry for a successful call", err)
	}
}

func TestCollectResource_FnHandledNotFoundSuppressesRecording(t *testing.T) {
	// Mirrors the CoreDNS ConfigMap / deprecated-API call sites in
	// Collect: fn can swallow a "not really a failure" case (e.g.
	// apierrors.IsNotFound) by returning nil itself -- collectResource must
	// not layer any error on top of that.
	snap := &Snapshot{Errors: map[string]error{}}
	c := &Collector{}

	c.collectResource(context.Background(), time.Second, &sync.Mutex{}, snap, "optional-resource", func(callCtx context.Context) error {
		return nil // fn decided this "not found" case isn't a real error
	})

	if err, ok := snap.Errors["optional-resource"]; ok {
		t.Errorf("Errors[\"optional-resource\"] = %v, want no entry when fn suppresses its own error", err)
	}
}

func TestCollectResource_NoGoroutineLeak(t *testing.T) {
	c := &Collector{}
	before := runtime.NumGoroutine()

	for i := 0; i < 50; i++ {
		snap := &Snapshot{Errors: map[string]error{}}
		c.collectResource(context.Background(), 5*time.Millisecond, &sync.Mutex{}, snap, "leak-check", func(callCtx context.Context) error {
			<-callCtx.Done()
			return callCtx.Err()
		})
	}

	// Allow the runtime a moment to settle any deferred cleanup, then
	// compare -- collectResource's deferred cancel() should release the
	// context package's internal timer immediately on each call, well
	// before this check runs.
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Errorf("goroutine count grew from %d to %d after 50 timeout-bound calls, want it to stay flat", before, after)
	}
}
