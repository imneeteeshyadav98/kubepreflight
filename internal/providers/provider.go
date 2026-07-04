// Package providers identifies the cloud/cluster providers KubePreflight's
// CLI knows about (today: eks, aks, gke, or cluster-only when omitted).
//
// This is deliberately a thin package for now. AWS/EKS enrichment
// (internal/collectors/aws) is unaffected by this package and keeps
// working exactly as it does today — that collection path is not routed
// through Provider. AKS and GKE are placeholders: they validate their
// CLI flags but their actual enrichment collection isn't built yet (see
// docs/provider-roadmap.md). Data-collection methods (cluster version,
// node pool, upgrade insight, add-on compatibility, network headroom
// discovery) are added to this interface in a later phase, once a real
// AKS or GKE API response shape exists to design the return types
// against — inventing those types now, before there's a concrete API to
// match, would just be speculative churn.
package providers

// Provider identifies a supported cloud/cluster provider.
type Provider interface {
	// Name returns the provider's CLI identifier ("eks", "aks", "gke").
	Name() string
}
