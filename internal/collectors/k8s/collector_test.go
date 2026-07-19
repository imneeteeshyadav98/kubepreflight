package k8s_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/apicatalog"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/testutil"
)

func fixturesDir() string {
	return filepath.Join("..", "..", "..", "testdata", "fixtures")
}

func TestCollector_Collect(t *testing.T) {
	objs, err := testutil.LoadFixtures(fixturesDir())
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	coreObjs, crdObjs := testutil.SplitCRDs(objs)

	client := fake.NewSimpleClientset(coreObjs...)
	apiExtCli := apiextensionsfake.NewSimpleClientset(crdObjs...)

	dynamicClient := testutil.NewFakeDynamicClient()

	c := k8s.NewCollector(client, apiExtCli, dynamicClient)
	snap, err := c.Collect(context.Background(), k8s.DefaultCollectorTimeout, k8s.DefaultCollectorConcurrency)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}

	cases := map[string]int{
		"Nodes":                     len(snap.Nodes),
		"Pods":                      len(snap.Pods),
		"PodDisruptionBudgets":      len(snap.PodDisruptionBudgets),
		"ValidatingWebhookConfigs":  len(snap.ValidatingWebhookConfigs),
		"Services":                  len(snap.Services),
		"EndpointSlices":            len(snap.EndpointSlices),
		"CustomResourceDefinitions": len(snap.CustomResourceDefinitions),
		"Deployments":               len(snap.Deployments),
		"DaemonSets":                len(snap.DaemonSets),
		"StatefulSets":              len(snap.StatefulSets),
		"PersistentVolumes":         len(snap.PersistentVolumes),
		"PersistentVolumeClaims":    len(snap.PersistentVolumeClaims),
		"DeprecatedAPIUsage":        len(snap.DeprecatedAPIUsage),
	}
	want := map[string]int{
		"Nodes":                     1,
		"Pods":                      2,
		"PodDisruptionBudgets":      1,
		"ValidatingWebhookConfigs":  1,
		"Services":                  1,
		"EndpointSlices":            1,
		"CustomResourceDefinitions": 1,
		"Deployments":               1,
		"DaemonSets":                1,
		"StatefulSets":              0,
		"PersistentVolumes":         0,
		"PersistentVolumeClaims":    0,
		"DeprecatedAPIUsage":        0,
	}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s = %d, want %d", name, got, want[name])
		}
	}

	if got := snap.CustomResourceDefinitions[0].Status.StoredVersions; len(got) != 1 || got[0] != "v1" {
		t.Errorf("CRD storedVersions = %v, want [v1]", got)
	}
	if got := snap.EndpointSlices[0].Endpoints; len(got) != 0 {
		t.Errorf("expected fixture EndpointSlice to have zero endpoints (WH-002 scenario), got %d", len(got))
	}
	if snap.CoreDNSConfigMap != nil {
		t.Errorf("CoreDNSConfigMap = %+v, want nil (fixture set has no kube-system/coredns ConfigMap)", snap.CoreDNSConfigMap)
	}
}

func TestCollector_Collect_CoreDNSConfigMapAllowlistedGet(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "fixtures", "checks", "coredns001", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	coreObjs, crdObjs := testutil.SplitCRDs(objs)

	client := fake.NewSimpleClientset(coreObjs...)
	apiExtCli := apiextensionsfake.NewSimpleClientset(crdObjs...)
	dynamicClient := testutil.NewFakeDynamicClient()

	c := k8s.NewCollector(client, apiExtCli, dynamicClient)
	snap, err := c.Collect(context.Background(), k8s.DefaultCollectorTimeout, k8s.DefaultCollectorConcurrency)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}
	if snap.CoreDNSConfigMap == nil {
		t.Fatal("CoreDNSConfigMap is nil, want the fixture ConfigMap to be picked up via the allowlisted Get")
	}
	if snap.CoreDNSConfigMap.Name != "coredns" || snap.CoreDNSConfigMap.Namespace != "kube-system" {
		t.Errorf("CoreDNSConfigMap = %s/%s, want kube-system/coredns", snap.CoreDNSConfigMap.Namespace, snap.CoreDNSConfigMap.Name)
	}
}

func TestCollector_Collect_StatefulSetsPVsPVCs(t *testing.T) {
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "default"}}
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "local-pv-1"}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "cache-data", Namespace: "default"}}

	client := fake.NewSimpleClientset(sts, pv, pvc)
	apiExtCli := apiextensionsfake.NewSimpleClientset()
	dynamicClient := testutil.NewFakeDynamicClient()

	c := k8s.NewCollector(client, apiExtCli, dynamicClient)
	snap, err := c.Collect(context.Background(), k8s.DefaultCollectorTimeout, k8s.DefaultCollectorConcurrency)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}
	if len(snap.StatefulSets) != 1 || snap.StatefulSets[0].Name != "cache" {
		t.Errorf("StatefulSets = %+v, want one named cache", snap.StatefulSets)
	}
	if len(snap.PersistentVolumes) != 1 || snap.PersistentVolumes[0].Name != "local-pv-1" {
		t.Errorf("PersistentVolumes = %+v, want one named local-pv-1", snap.PersistentVolumes)
	}
	if len(snap.PersistentVolumeClaims) != 1 || snap.PersistentVolumeClaims[0].Name != "cache-data" {
		t.Errorf("PersistentVolumeClaims = %+v, want one named cache-data", snap.PersistentVolumeClaims)
	}
}

// TestCollector_Collect_OneFailureDoesNotBlockOthers exercises the same
// "never all-or-nothing" invariant collectResource is built to preserve --
// see the doc comment on collectResource in collector.go -- using error
// injection rather than an actual timeout, since k8s.io/client-go's fake
// clientset never threads context into its reactor chain (see
// collector_timeout_test.go's package comment) and so has no way to
// simulate a hung call. A reactor-returned error and a context-deadline
// error both flow through the exact same collectResource call sites in
// Collect, so this still proves the wiring: one resource kind failing
// doesn't prevent the others in the same Collect() call from succeeding.
func TestCollector_Collect_OneFailureDoesNotBlockOthers(t *testing.T) {
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}},
	)
	client.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, k8sruntime.Object, error) {
		return true, nil, errors.New("simulated nodes list failure")
	})
	apiExtCli := apiextensionsfake.NewSimpleClientset()
	dynamicClient := testutil.NewFakeDynamicClient()

	c := k8s.NewCollector(client, apiExtCli, dynamicClient)
	snap, err := c.Collect(context.Background(), k8s.DefaultCollectorTimeout, k8s.DefaultCollectorConcurrency)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	nodesErr, ok := snap.Errors["nodes"]
	if !ok || nodesErr == nil {
		t.Fatal("Errors[\"nodes\"] not set, want the injected failure recorded")
	}
	if len(snap.Errors) != 1 {
		t.Errorf("Errors = %+v, want only \"nodes\" to have failed", snap.Errors)
	}
	if len(snap.Deployments) != 1 || snap.Deployments[0].Name != "app" {
		t.Errorf("Deployments = %+v, want the Deployment collected despite the Nodes failure", snap.Deployments)
	}
}

// TestCollector_ServerVersion guards the plan command's --from-version=auto
// discovery path for cluster-only (non-EKS) runs, which has no other way
// to learn the cluster's current version.
func TestCollector_ServerVersion(t *testing.T) {
	client := fake.NewSimpleClientset()
	fakeDisco, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
	if !ok {
		t.Fatal("fake clientset's Discovery() is not *fakediscovery.FakeDiscovery")
	}
	fakeDisco.FakedServerVersion = &version.Info{GitVersion: "v1.29.6-eks-1234567"}

	c := k8s.NewCollector(client, apiextensionsfake.NewSimpleClientset(), testutil.NewFakeDynamicClient())
	got, err := c.ServerVersion(context.Background(), k8s.DefaultCollectorTimeout)
	if err != nil {
		t.Fatalf("ServerVersion: %v", err)
	}
	if got != "v1.29.6-eks-1234567" {
		t.Errorf("ServerVersion() = %q, want %q", got, "v1.29.6-eks-1234567")
	}
}

func TestIsAutoManagedObject(t *testing.T) {
	flowSchema := apicatalog.DeprecatedAPI{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta1", Kind: "FlowSchema"}
	endpointSlice := apicatalog.DeprecatedAPI{Group: "discovery.k8s.io", Version: "v1beta1", Kind: "EndpointSlice"}
	other := apicatalog.DeprecatedAPI{Group: "policy", Version: "v1beta1", Kind: "PodSecurityPolicy"}

	cases := []struct {
		name string
		dep  apicatalog.DeprecatedAPI
		obj  unstructured.Unstructured
		want bool
	}{
		{
			name: "flowcontrol default carries the real autoupdate-spec annotation",
			dep:  flowSchema,
			obj:  unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"annotations": map[string]interface{}{"apf.kubernetes.io/autoupdate-spec": "true"}}}},
			want: true,
		},
		{
			name: "user-created FlowSchema has no annotation",
			dep:  flowSchema,
			obj:  unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{}}},
			want: false,
		},
		{
			// Confirmed against a real EKS 1.31 cluster: eks-exempt,
			// eks-leader-election, eks-monitoring, and eks-workload-high
			// all carry apf.kubernetes.io/autoupdate-spec: "false" (or no
			// annotation at all) but are apply-patched by field manager
			// "eks-internal" -- EKS's own control plane, not a person.
			name: "EKS-injected FlowSchema has autoupdate-spec false but the eks-internal field manager",
			dep:  flowSchema,
			obj: func() unstructured.Unstructured {
				u := unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"annotations": map[string]interface{}{"apf.kubernetes.io/autoupdate-spec": "false"}}}}
				u.SetManagedFields([]metav1.ManagedFieldsEntry{
					{Manager: "api-priority-and-fairness-config-consumer-v1", Operation: metav1.ManagedFieldsOperationApply},
					{Manager: "eks-internal", Operation: metav1.ManagedFieldsOperationApply},
				})
				return u
			}(),
			want: true,
		},
		{
			// eks-monitoring (both FlowSchema and PriorityLevelConfiguration)
			// carries no autoupdate-spec annotation at all, unlike the
			// other three eks-* defaults -- the field manager is the only
			// signal available for this one.
			name: "EKS-injected PriorityLevelConfiguration has no annotation at all, only the eks-internal field manager",
			dep:  apicatalog.DeprecatedAPI{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta1", Kind: "PriorityLevelConfiguration"},
			obj: func() unstructured.Unstructured {
				u := unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{}}}
				u.SetManagedFields([]metav1.ManagedFieldsEntry{
					{Manager: "eks-internal", Operation: metav1.ManagedFieldsOperationApply},
				})
				return u
			}(),
			want: true,
		},
		{
			name: "controller-managed EndpointSlice carries the real managed-by label",
			dep:  endpointSlice,
			obj: endpointSliceObject(
				map[string]string{"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io"},
				[]metav1.OwnerReference{{Kind: "Service", Name: "checkout", Controller: boolPtr(true)}},
			),
			want: true,
		},
		{
			name: "the default/kubernetes EndpointSlice exception has no managed-by label",
			dep:  endpointSlice,
			obj:  unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"kubernetes.io/service-name": "kubernetes"}}}},
			want: false,
		},
		{
			name: "a GVK this function doesn't special-case is never auto-managed",
			dep:  other,
			obj:  unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"annotations": map[string]interface{}{"apf.kubernetes.io/autoupdate-spec": "true"}}}},
			want: false,
		},
		{
			// Regression guard: eks-internal must never become a blanket
			// "trust this field manager" bypass. It's only consulted inside
			// the flowcontrol.apiserver.k8s.io case -- an arbitrary object
			// of some other GVK (a PodSecurityPolicy here, but this stands
			// for any resource kind) apply-patched by a manager happening to
			// be named "eks-internal" gets no exemption at all.
			name: "eks-internal field manager on a non-flowcontrol GVK grants no exemption",
			dep:  other,
			obj: func() unstructured.Unstructured {
				u := unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{}}}
				u.SetManagedFields([]metav1.ManagedFieldsEntry{
					{Manager: "eks-internal", Operation: metav1.ManagedFieldsOperationApply},
				})
				return u
			}(),
			want: false,
		},
		{
			// Regression guard against name-matching: an object literally
			// named "eks-exempt" (the real EKS default's own name) with
			// neither the annotation nor the eks-internal field manager is
			// exactly as user-owned as any other FlowSchema -- this
			// function was never told to trust a name, only the two real
			// lifecycle signals, and must keep not doing so.
			name: "an object merely named like a real EKS default, without the field manager or annotation, is not auto-managed",
			dep:  flowSchema,
			obj:  unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"name": "eks-exempt"}}},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := k8s.IsAutoManagedObject(tc.dep, tc.obj); got != tc.want {
				t.Errorf("IsAutoManagedObject(%s/%s) = %v, want %v", tc.dep.Group, tc.dep.Kind, got, tc.want)
			}
		})
	}
}

func TestIsAutoManagedObject_EndpointSliceSpoofingNearMatches(t *testing.T) {
	endpointSlice := apicatalog.DeprecatedAPI{Group: "discovery.k8s.io", Version: "v1beta1", Kind: "EndpointSlice"}
	other := apicatalog.DeprecatedAPI{Group: "policy", Version: "v1beta1", Kind: "PodSecurityPolicy"}

	cases := []struct {
		name string
		dep  apicatalog.DeprecatedAPI
		obj  unstructured.Unstructured
		want bool
	}{
		{
			name: "valid exact controller label plus Service controller owner",
			dep:  endpointSlice,
			obj: endpointSliceObject(
				map[string]string{"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io"},
				[]metav1.OwnerReference{{Kind: "Service", Name: "checkout", Controller: boolPtr(true)}},
			),
			want: true,
		},
		{
			name: "managed-by label only is not enough",
			dep:  endpointSlice,
			obj: endpointSliceObject(
				map[string]string{"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io"},
				nil,
			),
			want: false,
		},
		{
			name: "Service owner only is not enough",
			dep:  endpointSlice,
			obj: endpointSliceObject(
				nil,
				[]metav1.OwnerReference{{Kind: "Service", Name: "checkout", Controller: boolPtr(true)}},
			),
			want: false,
		},
		{
			name: "wrong managed-by value is not enough even with Service owner",
			dep:  endpointSlice,
			obj: endpointSliceObject(
				map[string]string{"endpointslice.kubernetes.io/managed-by": "attacker-controller"},
				[]metav1.OwnerReference{{Kind: "Service", Name: "checkout", Controller: boolPtr(true)}},
			),
			want: false,
		},
		{
			name: "non-controller Service owner is not enough",
			dep:  endpointSlice,
			obj: endpointSliceObject(
				map[string]string{"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io"},
				[]metav1.OwnerReference{{Kind: "Service", Name: "checkout", Controller: boolPtr(false)}},
			),
			want: false,
		},
		{
			name: "wrong owner kind is not enough",
			dep:  endpointSlice,
			obj: endpointSliceObject(
				map[string]string{"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io"},
				[]metav1.OwnerReference{{Kind: "Deployment", Name: "checkout", Controller: boolPtr(true)}},
			),
			want: false,
		},
		{
			name: "unsupported GVK with EndpointSlice signals is not exempt",
			dep:  other,
			obj: endpointSliceObject(
				map[string]string{"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io"},
				[]metav1.OwnerReference{{Kind: "Service", Name: "checkout", Controller: boolPtr(true)}},
			),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := k8s.IsAutoManagedObject(tc.dep, tc.obj); got != tc.want {
				t.Errorf("IsAutoManagedObject(%s/%s) = %v, want %v", tc.dep.Group, tc.dep.Kind, got, tc.want)
			}
		})
	}
}

func endpointSliceObject(labels map[string]string, owners []metav1.OwnerReference) unstructured.Unstructured {
	u := unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{}}}
	u.SetLabels(labels)
	u.SetOwnerReferences(owners)
	return u
}

func boolPtr(v bool) *bool {
	return &v
}
