package cli

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"
)

type rbacManifest struct {
	Kind  string `json:"kind"`
	Rules []struct {
		APIGroups     []string `json:"apiGroups"`
		Resources     []string `json:"resources"`
		ResourceNames []string `json:"resourceNames"`
		Verbs         []string `json:"verbs"`
	} `json:"rules"`
}

func TestDeployClusterRoleContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "clusterrole.yaml"))
	if err != nil {
		t.Fatalf("read deploy/clusterrole.yaml: %v", err)
	}

	docs := splitYAMLDocuments(string(raw))
	var manifests []rbacManifest
	for _, doc := range docs {
		if doc == "" {
			continue
		}
		var manifest rbacManifest
		if err := yaml.Unmarshal([]byte(doc), &manifest); err != nil {
			t.Fatalf("parse RBAC manifest: %v", err)
		}
		if manifest.Kind == "ClusterRole" || manifest.Kind == "Role" {
			manifests = append(manifests, manifest)
		}
	}
	if len(manifests) == 0 {
		t.Fatal("no ClusterRole/Role documents found")
	}

	granted := map[string]bool{}
	for _, manifest := range manifests {
		for _, rule := range manifest.Rules {
			for _, verb := range rule.Verbs {
				if verb != "get" && verb != "list" {
					t.Fatalf("%s grants non-read verb %q", manifest.Kind, verb)
				}
			}
			for _, resource := range rule.Resources {
				if resource == "*" {
					t.Fatalf("%s grants wildcard resource", manifest.Kind)
				}
				if resource == "secrets" {
					t.Fatalf("%s grants secrets access", manifest.Kind)
				}
				for _, group := range rule.APIGroups {
					if group == "*" {
						t.Fatalf("%s grants wildcard apiGroup", manifest.Kind)
					}
					granted[group+"/"+resource] = true
				}
			}
		}
	}

	for _, required := range []string{
		"/nodes",
		"/pods",
		"/services",
		"/persistentvolumes",
		"/persistentvolumeclaims",
		"extensions/podsecuritypolicies",
		"storage.k8s.io/csistoragecapacities",
		"apiregistration.k8s.io/apiservices",
		"apiextensions.k8s.io/customresourcedefinitions",
	} {
		if !granted[required] {
			t.Errorf("deploy/clusterrole.yaml missing required read access for %s", required)
		}
	}
}

func splitYAMLDocuments(s string) []string {
	var docs []string
	start := 0
	for i := 0; i < len(s); i++ {
		if (i == 0 || s[i-1] == '\n') && i+3 <= len(s) && s[i:i+3] == "---" {
			docs = append(docs, s[start:i])
			for i < len(s) && s[i] != '\n' {
				i++
			}
			start = i + 1
		}
	}
	docs = append(docs, s[start:])
	return docs
}
