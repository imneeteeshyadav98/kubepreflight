package k8s_test

import (
	"context"
	"path/filepath"
	"testing"

	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/client-go/kubernetes/fake"

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
