// Package aks identifies the AKS provider for CLI wiring. Enrichment
// collection (cluster/node-pool version discovery, available upgrades,
// Azure CNI subnet/IP headroom, add-on profile compatibility) isn't
// implemented yet — see docs/provider-roadmap.md. This package only
// validates the CLI flags --provider=aks requires.
package aks

import "fmt"

// AKS identifies the AKS provider.
type AKS struct{}

// Name returns "aks".
func (AKS) Name() string { return "aks" }

// Config holds the flags required to identify an AKS cluster.
// SubscriptionID is optional — when empty, callers fall back to
// whatever the Azure CLI/SDK's default credential chain resolves.
type Config struct {
	ClusterName    string
	ResourceGroup  string
	SubscriptionID string
}

// Validate checks that Config has every field --provider=aks requires.
func (c Config) Validate() error {
	if c.ClusterName == "" {
		return fmt.Errorf("--cluster-name is required when --provider=aks")
	}
	if c.ResourceGroup == "" {
		return fmt.Errorf("--resource-group is required when --provider=aks")
	}
	return nil
}
