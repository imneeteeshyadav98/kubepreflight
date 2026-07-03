// Package apicatalog holds the deprecated/removed Kubernetes API ruleset
// used by API-001. It is intentionally dependency-free data: both the
// k8s collector (which needs to know what GVRs to list) and the rules
// package (which needs to know when an entry applies to a target version)
// import it, so it can't live inside either without creating a cycle.
//
// Adding a newly-removed API here is a data change, never a code change.
package apicatalog

import "k8s.io/apimachinery/pkg/runtime/schema"

// DeprecatedAPI describes one Kubernetes API group/version/resource that
// the API server stops serving entirely as of RemovedInVersion.
type DeprecatedAPI struct {
	Group            string
	Version          string
	Resource         string // plural resource name, e.g. "podsecuritypolicies"
	Kind             string // display kind, e.g. "PodSecurityPolicy"
	RemovedInVersion string // first Kubernetes minor version that no longer serves this GVR, e.g. "1.25"
	Replacement      string
}

// Deprecated is the known-removals ruleset. Historic entries per the deep
// dive Section 4.1; not exhaustive by design for v0.1.
var Deprecated = []DeprecatedAPI{
	{
		Group: "policy", Version: "v1beta1", Resource: "podsecuritypolicies", Kind: "PodSecurityPolicy",
		RemovedInVersion: "1.25", Replacement: "Pod Security Admission or a policy engine (Kyverno/Gatekeeper)",
	},
	{
		Group: "extensions", Version: "v1beta1", Resource: "deployments", Kind: "Deployment",
		RemovedInVersion: "1.16", Replacement: "apps/v1 Deployment",
	},
	{
		Group: "extensions", Version: "v1beta1", Resource: "daemonsets", Kind: "DaemonSet",
		RemovedInVersion: "1.16", Replacement: "apps/v1 DaemonSet",
	},
	{
		Group: "autoscaling", Version: "v2beta2", Resource: "horizontalpodautoscalers", Kind: "HorizontalPodAutoscaler",
		RemovedInVersion: "1.26", Replacement: "autoscaling/v2 HorizontalPodAutoscaler",
	},
}

// GVR returns the GroupVersionResource this entry describes.
func (d DeprecatedAPI) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: d.Group, Version: d.Version, Resource: d.Resource}
}

// ListKind is the List-suffixed kind name dynamic clients need to know how
// to list this GVR, since the type itself may no longer be registered in
// any Go scheme (some of these structs are gone from k8s.io/api entirely).
func (d DeprecatedAPI) ListKind() string {
	return d.Kind + "List"
}

// GVRToListKind builds the map fake dynamic clients need to serve List
// calls for GVRs with no scheme registration.
func GVRToListKind() map[schema.GroupVersionResource]string {
	m := make(map[schema.GroupVersionResource]string, len(Deprecated))
	for _, d := range Deprecated {
		m[d.GVR()] = d.ListKind()
	}
	return m
}
