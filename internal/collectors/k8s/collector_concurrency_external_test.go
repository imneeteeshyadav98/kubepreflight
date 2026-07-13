package k8s_test

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"k8s.io/client-go/kubernetes/fake"

	"kubepreflight/internal/apicatalog"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/testutil"
)

// deprecatedObj builds a synthetic unstructured object at dep's exact
// GVK, matching what the fake dynamic client needs to serve it back from
// Resource(dep.GVR()).List(...) -- the same code path Collect's
// deprecated-API fan-out goes through.
func deprecatedObj(dep apicatalog.DeprecatedAPI, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": dep.Group + "/" + dep.Version,
		"kind":       dep.Kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
	}}
}

// findDeprecated looks up a single apicatalog.Deprecated entry by group and
// kind -- used instead of a hardcoded slice index so this test doesn't
// silently start exercising a different GVR if apicatalog.Deprecated is
// ever reordered.
func findDeprecated(t *testing.T, group, kind string) apicatalog.DeprecatedAPI {
	t.Helper()
	for _, dep := range apicatalog.Deprecated {
		if dep.Group == group && dep.Kind == kind {
			return dep
		}
	}
	t.Fatalf("no apicatalog.Deprecated entry for group %q kind %q", group, kind)
	return apicatalog.DeprecatedAPI{}
}

// deprecatedAPIUsageFixtures returns several deprecated-API objects spread
// across four distinct GVRs (some with more than one object), so that
// Collect's per-GVR fan-out genuinely has multiple tasks all appending into
// the same Snapshot.DeprecatedAPIUsage slice -- the one place concurrency
// could plausibly produce different orderings across runs. All four GVRs
// are ones k8s.io/client-go's own scheme has no registered Go type for
// (confirmed via scheme.Recognizes: PodSecurityPolicy at either of its two
// removed group/versions, CustomResourceDefinition v1beta1, and APIService
// v1beta1) -- deliberately avoiding GVRs like extensions/v1beta1 Deployment
// that DO still have a registered legacy type in client-go's scheme for
// backward compat, which makes k8s.io/client-go/dynamic/fake's object
// tracker attempt a strict typed conversion that a hand-built minimal
// unstructured object fails ("can't assign or convert
// unstructured.Unstructured into v1beta1.Deployment") -- a fake-dynamic-
// client-in-tests artifact, not a real Collect() bug: a real cluster's
// REST response decodes through a completely different path.
func deprecatedAPIUsageFixtures(t *testing.T) (objs []runtime.Object, deps []apicatalog.DeprecatedAPI) {
	pspPolicy := findDeprecated(t, "policy", "PodSecurityPolicy")
	pspExtensions := findDeprecated(t, "extensions", "PodSecurityPolicy")
	crd := findDeprecated(t, "apiextensions.k8s.io", "CustomResourceDefinition")
	apiService := findDeprecated(t, "apiregistration.k8s.io", "APIService")

	fixtures := []*unstructured.Unstructured{
		deprecatedObj(pspPolicy, "", "restricted"),
		deprecatedObj(pspPolicy, "", "baseline"),
		deprecatedObj(pspExtensions, "", "legacy-restricted"),
		deprecatedObj(crd, "", "widgets.example.com"),
		deprecatedObj(crd, "", "gadgets.example.com"),
		deprecatedObj(apiService, "", "v1beta1.metrics.k8s.io"),
	}
	for _, f := range fixtures {
		objs = append(objs, f)
	}
	return objs, []apicatalog.DeprecatedAPI{pspPolicy, pspExtensions, crd, apiService}
}

func newConcurrencyTestCollector(t *testing.T) *k8s.Collector {
	t.Helper()
	deprecatedObjs, _ := deprecatedAPIUsageFixtures(t)

	client := fake.NewSimpleClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}},
	)
	apiExtCli := apiextensionsfake.NewSimpleClientset()
	dynamicClient := testutil.NewFakeDynamicClient(deprecatedObjs...)

	return k8s.NewCollector(client, apiExtCli, dynamicClient)
}

func sortedDeprecatedAPIUsage(items []k8s.DeprecatedAPIObject) []k8s.DeprecatedAPIObject {
	out := append([]k8s.DeprecatedAPIObject(nil), items...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	return out
}

// TestCollector_Collect_DeterministicAcrossConcurrencyLevels is the direct
// test of this PR's central correctness claim: concurrency must never
// change WHAT Collect finds, only (for the one slice multiple tasks append
// into) the order items land in. Errors and every single-assignment field
// are compared byte-for-byte; DeprecatedAPIUsage is compared after a
// canonical sort, exactly the same normalization the report layer already
// applies to finding output before rendering.
func TestCollector_Collect_DeterministicAcrossConcurrencyLevels(t *testing.T) {
	var reference []k8s.DeprecatedAPIObject
	var referenceErrCount int

	for _, concurrency := range []int{1, 2, 4, 8} {
		c := newConcurrencyTestCollector(t)
		snap, err := c.Collect(context.Background(), k8s.DefaultCollectorTimeout, concurrency)
		if err != nil {
			t.Fatalf("concurrency=%d: Collect returned error: %v", concurrency, err)
		}
		if len(snap.Errors) != 0 {
			t.Fatalf("concurrency=%d: unexpected collector errors: %v", concurrency, snap.Errors)
		}

		got := sortedDeprecatedAPIUsage(snap.DeprecatedAPIUsage)
		if len(got) != 6 {
			t.Fatalf("concurrency=%d: got %d DeprecatedAPIUsage items, want 6", concurrency, len(got))
		}
		if reference == nil {
			reference = got
			referenceErrCount = len(snap.Errors)
			continue
		}
		if !reflect.DeepEqual(got, reference) {
			t.Errorf("concurrency=%d: sorted DeprecatedAPIUsage differs from concurrency=1's result\ngot:  %+v\nwant: %+v", concurrency, got, reference)
		}
		if len(snap.Errors) != referenceErrCount {
			t.Errorf("concurrency=%d: %d errors, want %d (same as concurrency=1)", concurrency, len(snap.Errors), referenceErrCount)
		}
		if len(snap.Nodes) != 1 || snap.Nodes[0].Name != "node-1" {
			t.Errorf("concurrency=%d: Nodes = %+v, want one named node-1", concurrency, snap.Nodes)
		}
		if len(snap.Deployments) != 1 || snap.Deployments[0].Name != "app" {
			t.Errorf("concurrency=%d: Deployments = %+v, want one named app", concurrency, snap.Deployments)
		}
	}
}

// TestCollector_Collect_OneFailureDoesNotBlockOthers_AtHigherConcurrency
// re-proves the existing sequential-mode "never all-or-nothing" invariant
// (see the pre-existing TestCollector_Collect_OneFailureDoesNotBlockOthers
// in collector_test.go) under real concurrent scheduling, not just
// concurrency=1. Uses error injection rather than an actual timeout for
// the same reason collector_test.go's version does: k8s.io/client-go's
// fake clientset never threads ctx into its reactor chain at all (see
// collector_timeout_test.go's package comment), so a reactor can't
// observe -- or be bounded by -- a context deadline; blocking a reactor on
// a channel would hang Collect forever rather than simulating a timeout,
// since the real List call it wraps never checks callCtx.Done() either. A
// reactor-returned error and a context-deadline error both flow through
// the exact same collectResource call site, so this still proves what
// matters here: one resource kind failing does not prevent the others
// from succeeding when several tasks are genuinely running at once.
func TestCollector_Collect_OneFailureDoesNotBlockOthers_AtHigherConcurrency(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "kube-system"}},
	)
	client.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated nodes list failure")
	})
	apiExtCli := apiextensionsfake.NewSimpleClientset()
	dynamicClient := testutil.NewFakeDynamicClient()

	c := k8s.NewCollector(client, apiExtCli, dynamicClient)
	snap, err := c.Collect(context.Background(), k8s.DefaultCollectorTimeout, 4)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	nodesErr, ok := snap.Errors["nodes"]
	if !ok || nodesErr == nil {
		t.Fatal(`Errors["nodes"] not set, want the injected failure recorded`)
	}
	if len(snap.Errors) != 1 {
		t.Errorf("Errors = %+v, want only \"nodes\" to have failed", snap.Errors)
	}
	if len(snap.Deployments) != 1 || snap.Deployments[0].Name != "app" {
		t.Errorf("Deployments = %+v, want one named app despite the nodes call failing", snap.Deployments)
	}
	if len(snap.DaemonSets) != 1 || snap.DaemonSets[0].Name != "agent" {
		t.Errorf("DaemonSets = %+v, want one named agent despite the nodes call failing", snap.DaemonSets)
	}
}
