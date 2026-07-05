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
	Namespaced       bool   // false means the resource is cluster-scoped
	RemovedInVersion string // first Kubernetes minor version that no longer serves this GVR, e.g. "1.25"
	Replacement      string

	// ReplacementAPIVersion is the bare "<group>/<version>" a straight
	// apiVersion swap moves to, e.g. "policy/v1". Left empty when the
	// replacement isn't a 1:1 version bump (e.g. PodSecurityPolicy, whose
	// replacement is a different admission mechanism entirely) — a diff
	// can only be honestly shown when this is set.
	ReplacementAPIVersion string
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
		Namespaced:       true,
		RemovedInVersion: "1.16", Replacement: "apps/v1 Deployment", ReplacementAPIVersion: "apps/v1",
	},
	{
		Group: "extensions", Version: "v1beta1", Resource: "daemonsets", Kind: "DaemonSet",
		Namespaced:       true,
		RemovedInVersion: "1.16", Replacement: "apps/v1 DaemonSet", ReplacementAPIVersion: "apps/v1",
	},
	{
		Group: "autoscaling", Version: "v2beta2", Resource: "horizontalpodautoscalers", Kind: "HorizontalPodAutoscaler",
		Namespaced:       true,
		RemovedInVersion: "1.26", Replacement: "autoscaling/v2 HorizontalPodAutoscaler", ReplacementAPIVersion: "autoscaling/v2",
	},
	// Found missing via a real live-EKS test (policy/v1beta1
	// PodDisruptionBudget didn't fire API-001) — added at the end, not
	// inserted earlier in the slice, since at least one existing test
	// indexes into Deprecated positionally; appending keeps that index
	// stable for any future addition too.
	{
		Group: "policy", Version: "v1beta1", Resource: "poddisruptionbudgets", Kind: "PodDisruptionBudget",
		Namespaced:       true,
		RemovedInVersion: "1.25", Replacement: "policy/v1 PodDisruptionBudget", ReplacementAPIVersion: "policy/v1",
	},
	{Group: "apps", Version: "v1beta1", Resource: "deployments", Kind: "Deployment", Namespaced: true, RemovedInVersion: "1.16", Replacement: "apps/v1 Deployment", ReplacementAPIVersion: "apps/v1"},
	{Group: "apps", Version: "v1beta2", Resource: "deployments", Kind: "Deployment", Namespaced: true, RemovedInVersion: "1.16", Replacement: "apps/v1 Deployment", ReplacementAPIVersion: "apps/v1"},
	{Group: "apps", Version: "v1beta2", Resource: "daemonsets", Kind: "DaemonSet", Namespaced: true, RemovedInVersion: "1.16", Replacement: "apps/v1 DaemonSet", ReplacementAPIVersion: "apps/v1"},
	{Group: "apps", Version: "v1beta2", Resource: "replicasets", Kind: "ReplicaSet", Namespaced: true, RemovedInVersion: "1.16", Replacement: "apps/v1 ReplicaSet", ReplacementAPIVersion: "apps/v1"},
	{Group: "extensions", Version: "v1beta1", Resource: "replicasets", Kind: "ReplicaSet", Namespaced: true, RemovedInVersion: "1.16", Replacement: "apps/v1 ReplicaSet", ReplacementAPIVersion: "apps/v1"},
	{Group: "extensions", Version: "v1beta1", Resource: "networkpolicies", Kind: "NetworkPolicy", Namespaced: true, RemovedInVersion: "1.16", Replacement: "networking.k8s.io/v1 NetworkPolicy", ReplacementAPIVersion: "networking.k8s.io/v1"},
	{Group: "apps", Version: "v1beta1", Resource: "statefulsets", Kind: "StatefulSet", Namespaced: true, RemovedInVersion: "1.16", Replacement: "apps/v1 StatefulSet", ReplacementAPIVersion: "apps/v1"},
	{Group: "apps", Version: "v1beta2", Resource: "statefulsets", Kind: "StatefulSet", Namespaced: true, RemovedInVersion: "1.16", Replacement: "apps/v1 StatefulSet", ReplacementAPIVersion: "apps/v1"},
	{Group: "extensions", Version: "v1beta1", Resource: "ingresses", Kind: "Ingress", Namespaced: true, RemovedInVersion: "1.22", Replacement: "networking.k8s.io/v1 Ingress", ReplacementAPIVersion: "networking.k8s.io/v1"},
	{Group: "networking.k8s.io", Version: "v1beta1", Resource: "ingresses", Kind: "Ingress", Namespaced: true, RemovedInVersion: "1.22", Replacement: "networking.k8s.io/v1 Ingress", ReplacementAPIVersion: "networking.k8s.io/v1"},
	{Group: "networking.k8s.io", Version: "v1beta1", Resource: "ingressclasses", Kind: "IngressClass", RemovedInVersion: "1.22", Replacement: "networking.k8s.io/v1 IngressClass", ReplacementAPIVersion: "networking.k8s.io/v1"},
	{Group: "apiextensions.k8s.io", Version: "v1beta1", Resource: "customresourcedefinitions", Kind: "CustomResourceDefinition", RemovedInVersion: "1.22", Replacement: "apiextensions.k8s.io/v1 CustomResourceDefinition", ReplacementAPIVersion: "apiextensions.k8s.io/v1"},
	{Group: "admissionregistration.k8s.io", Version: "v1beta1", Resource: "validatingwebhookconfigurations", Kind: "ValidatingWebhookConfiguration", RemovedInVersion: "1.22", Replacement: "admissionregistration.k8s.io/v1 ValidatingWebhookConfiguration", ReplacementAPIVersion: "admissionregistration.k8s.io/v1"},
	{Group: "admissionregistration.k8s.io", Version: "v1beta1", Resource: "mutatingwebhookconfigurations", Kind: "MutatingWebhookConfiguration", RemovedInVersion: "1.22", Replacement: "admissionregistration.k8s.io/v1 MutatingWebhookConfiguration", ReplacementAPIVersion: "admissionregistration.k8s.io/v1"},
	{Group: "apiregistration.k8s.io", Version: "v1beta1", Resource: "apiservices", Kind: "APIService", RemovedInVersion: "1.22", Replacement: "apiregistration.k8s.io/v1 APIService", ReplacementAPIVersion: "apiregistration.k8s.io/v1"},
	{Group: "certificates.k8s.io", Version: "v1beta1", Resource: "certificatesigningrequests", Kind: "CertificateSigningRequest", RemovedInVersion: "1.22", Replacement: "certificates.k8s.io/v1 CertificateSigningRequest", ReplacementAPIVersion: "certificates.k8s.io/v1"},
	{Group: "coordination.k8s.io", Version: "v1beta1", Resource: "leases", Kind: "Lease", Namespaced: true, RemovedInVersion: "1.22", Replacement: "coordination.k8s.io/v1 Lease", ReplacementAPIVersion: "coordination.k8s.io/v1"},
	{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Resource: "roles", Kind: "Role", Namespaced: true, RemovedInVersion: "1.22", Replacement: "rbac.authorization.k8s.io/v1 Role", ReplacementAPIVersion: "rbac.authorization.k8s.io/v1"},
	{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Resource: "rolebindings", Kind: "RoleBinding", Namespaced: true, RemovedInVersion: "1.22", Replacement: "rbac.authorization.k8s.io/v1 RoleBinding", ReplacementAPIVersion: "rbac.authorization.k8s.io/v1"},
	{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Resource: "clusterroles", Kind: "ClusterRole", RemovedInVersion: "1.22", Replacement: "rbac.authorization.k8s.io/v1 ClusterRole", ReplacementAPIVersion: "rbac.authorization.k8s.io/v1"},
	{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Resource: "clusterrolebindings", Kind: "ClusterRoleBinding", RemovedInVersion: "1.22", Replacement: "rbac.authorization.k8s.io/v1 ClusterRoleBinding", ReplacementAPIVersion: "rbac.authorization.k8s.io/v1"},
	{Group: "scheduling.k8s.io", Version: "v1beta1", Resource: "priorityclasses", Kind: "PriorityClass", RemovedInVersion: "1.22", Replacement: "scheduling.k8s.io/v1 PriorityClass", ReplacementAPIVersion: "scheduling.k8s.io/v1"},
	{Group: "storage.k8s.io", Version: "v1beta1", Resource: "csidrivers", Kind: "CSIDriver", RemovedInVersion: "1.22", Replacement: "storage.k8s.io/v1 CSIDriver", ReplacementAPIVersion: "storage.k8s.io/v1"},
	{Group: "storage.k8s.io", Version: "v1beta1", Resource: "csinodes", Kind: "CSINode", RemovedInVersion: "1.22", Replacement: "storage.k8s.io/v1 CSINode", ReplacementAPIVersion: "storage.k8s.io/v1"},
	{Group: "storage.k8s.io", Version: "v1beta1", Resource: "storageclasses", Kind: "StorageClass", RemovedInVersion: "1.22", Replacement: "storage.k8s.io/v1 StorageClass", ReplacementAPIVersion: "storage.k8s.io/v1"},
	{Group: "storage.k8s.io", Version: "v1beta1", Resource: "volumeattachments", Kind: "VolumeAttachment", RemovedInVersion: "1.22", Replacement: "storage.k8s.io/v1 VolumeAttachment", ReplacementAPIVersion: "storage.k8s.io/v1"},
	{Group: "batch", Version: "v1beta1", Resource: "cronjobs", Kind: "CronJob", Namespaced: true, RemovedInVersion: "1.25", Replacement: "batch/v1 CronJob", ReplacementAPIVersion: "batch/v1"},
	{Group: "discovery.k8s.io", Version: "v1beta1", Resource: "endpointslices", Kind: "EndpointSlice", Namespaced: true, RemovedInVersion: "1.25", Replacement: "discovery.k8s.io/v1 EndpointSlice", ReplacementAPIVersion: "discovery.k8s.io/v1"},
	{Group: "events.k8s.io", Version: "v1beta1", Resource: "events", Kind: "Event", Namespaced: true, RemovedInVersion: "1.25", Replacement: "events.k8s.io/v1 Event", ReplacementAPIVersion: "events.k8s.io/v1"},
	{Group: "node.k8s.io", Version: "v1beta1", Resource: "runtimeclasses", Kind: "RuntimeClass", RemovedInVersion: "1.25", Replacement: "node.k8s.io/v1 RuntimeClass", ReplacementAPIVersion: "node.k8s.io/v1"},
	{Group: "autoscaling", Version: "v2beta1", Resource: "horizontalpodautoscalers", Kind: "HorizontalPodAutoscaler", Namespaced: true, RemovedInVersion: "1.25", Replacement: "autoscaling/v2 HorizontalPodAutoscaler", ReplacementAPIVersion: "autoscaling/v2"},
	{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta1", Resource: "flowschemas", Kind: "FlowSchema", RemovedInVersion: "1.26", Replacement: "flowcontrol.apiserver.k8s.io/v1 FlowSchema", ReplacementAPIVersion: "flowcontrol.apiserver.k8s.io/v1"},
	{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta1", Resource: "prioritylevelconfigurations", Kind: "PriorityLevelConfiguration", RemovedInVersion: "1.26", Replacement: "flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration", ReplacementAPIVersion: "flowcontrol.apiserver.k8s.io/v1"},
	{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta2", Resource: "flowschemas", Kind: "FlowSchema", RemovedInVersion: "1.29", Replacement: "flowcontrol.apiserver.k8s.io/v1 FlowSchema", ReplacementAPIVersion: "flowcontrol.apiserver.k8s.io/v1"},
	{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta2", Resource: "prioritylevelconfigurations", Kind: "PriorityLevelConfiguration", RemovedInVersion: "1.29", Replacement: "flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration", ReplacementAPIVersion: "flowcontrol.apiserver.k8s.io/v1"},
	{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta3", Resource: "flowschemas", Kind: "FlowSchema", RemovedInVersion: "1.32", Replacement: "flowcontrol.apiserver.k8s.io/v1 FlowSchema", ReplacementAPIVersion: "flowcontrol.apiserver.k8s.io/v1"},
	{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta3", Resource: "prioritylevelconfigurations", Kind: "PriorityLevelConfiguration", RemovedInVersion: "1.32", Replacement: "flowcontrol.apiserver.k8s.io/v1 PriorityLevelConfiguration", ReplacementAPIVersion: "flowcontrol.apiserver.k8s.io/v1"},
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
	// The Kubernetes collector also uses its dynamic client for current
	// APIService availability; fake clients need the list-kind registration.
	m[schema.GroupVersionResource{Group: "apiregistration.k8s.io", Version: "v1", Resource: "apiservices"}] = "APIServiceList"
	return m
}
