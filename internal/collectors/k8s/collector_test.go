package k8s_test

import (
	"context"
	"path/filepath"
	"testing"

	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"

	"kubepreflight/internal/apicatalog"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/testutil"
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
	snap, err := c.Collect(context.Background())
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
	snap, err := c.Collect(context.Background())
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
	got, err := c.ServerVersion()
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
			name: "controller-managed EndpointSlice carries the real managed-by label",
			dep:  endpointSlice,
			obj:  unstructured.Unstructured{Object: map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io"}}}},
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := k8s.IsAutoManagedObject(tc.dep, tc.obj); got != tc.want {
				t.Errorf("IsAutoManagedObject(%s/%s) = %v, want %v", tc.dep.Group, tc.dep.Kind, got, tc.want)
			}
		})
	}
}
