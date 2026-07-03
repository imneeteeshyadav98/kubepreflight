package findings

import "testing"

func TestFilterByNamespaceAllowlist(t *testing.T) {
	namespaced := func(namespace, name string) Finding {
		ref := LiveResource("Deployment", ScopeNamespaced, namespace, name, "uid-"+name)
		return Finding{RuleID: "TEST-001", Resources: []ResourceReference{ref}}
	}
	clusterRef := LiveResource("Node", ScopeCluster, "", "node-a", "uid-node")
	awsRef := AWSResource("Subnet", "subnet-a", "subnet-a")
	omitted := ManifestResource("Deployment", ScopeNamespaced, "", "unknown-ns", "deployment.yaml")
	mixed := Finding{RuleID: "TEST-002", Resources: []ResourceReference{
		LiveResource("ConfigMap", ScopeNamespaced, "payments", "a", "uid-a"),
		LiveResource("ConfigMap", ScopeNamespaced, "staging", "b", "uid-b"),
	}}

	input := []Finding{
		namespaced("payments", "kept"),
		namespaced("staging", "excluded"),
		{RuleID: "TEST-003", Resources: []ResourceReference{clusterRef}},
		{RuleID: "TEST-004", Resources: []ResourceReference{awsRef}},
		{RuleID: "TEST-005", Resources: []ResourceReference{omitted}},
		mixed,
	}
	got := FilterByNamespaceAllowlist(input, []string{"payments"})
	if len(got) != 3 {
		t.Fatalf("got %d findings, want allowed namespace + cluster + AWS: %+v", len(got), got)
	}
	if got[0].Resources[0].Name != "kept" || got[1].Resources[0].Kind != "Node" || got[2].Resources[0].Kind != "Subnet" {
		t.Errorf("unexpected filtered findings: %+v", got)
	}
}

func TestFilterByNamespaceAllowlist_EmptyAllowlistDoesNotFilter(t *testing.T) {
	input := []Finding{{RuleID: "TEST", Resources: []ResourceReference{
		LiveResource("Deployment", ScopeNamespaced, "staging", "api", "uid-api"),
	}}}
	got := FilterByNamespaceAllowlist(input, nil)
	if len(got) != 1 {
		t.Fatalf("empty allowlist filtered findings: %+v", got)
	}
}
