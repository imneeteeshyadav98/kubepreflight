package k8s_test

import (
	"context"
	"testing"

	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/imneeteeshyadav98/kubepreflight/internal/apicatalog"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/testutil"
)

func TestCollector_Collect_DoesNotMultiplyRequests(t *testing.T) {
	client := fake.NewSimpleClientset()
	apiExtCli := apiextensionsfake.NewSimpleClientset()
	dynamicClient := testutil.NewFakeDynamicClient()

	c := k8s.NewCollector(client, apiExtCli, dynamicClient)
	snap, err := c.Collect(context.Background(), k8s.DefaultCollectorTimeout, k8s.DefaultCollectorConcurrency)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}

	const typedClientCalls = 13
	if got := len(client.Actions()); got != typedClientCalls {
		t.Fatalf("typed Kubernetes client actions = %d, want %d fixed one-shot list/get calls", got, typedClientCalls)
	}
	if got := len(apiExtCli.Actions()); got != 1 {
		t.Fatalf("apiextensions client actions = %d, want 1 CRD list call", got)
	}
	dyn, ok := dynamicClient.(*dynamicfake.FakeDynamicClient)
	if !ok {
		t.Fatalf("dynamic client type = %T, want *dynamicfake.FakeDynamicClient", dynamicClient)
	}
	wantDynamicCalls := len(apicatalog.Deprecated) + 1 // every deprecated GVR plus apiregistration.k8s.io/v1 APIService inventory
	if got := len(dyn.Actions()); got != wantDynamicCalls {
		t.Fatalf("dynamic client actions = %d, want %d one-shot list calls", got, wantDynamicCalls)
	}
}
