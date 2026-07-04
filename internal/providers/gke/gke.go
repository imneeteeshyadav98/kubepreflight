// Package gke identifies the GKE provider for CLI wiring. Enrichment
// collection (cluster/node-pool version discovery, release channels,
// deprecation insights/recommendations, VPC-native secondary-range
// headroom, Autopilot vs. Standard behavior) isn't implemented yet — see
// docs/provider-roadmap.md. This package only validates the CLI flags
// --provider=gke requires.
package gke

import "fmt"

// GKE identifies the GKE provider.
type GKE struct{}

// Name returns "gke".
func (GKE) Name() string { return "gke" }

// Config holds the flags required to identify a GKE cluster.
type Config struct {
	ClusterName string
	Project     string
	Location    string
}

// Validate checks that Config has every field --provider=gke requires.
func (c Config) Validate() error {
	if c.ClusterName == "" {
		return fmt.Errorf("--cluster-name is required when --provider=gke")
	}
	if c.Project == "" {
		return fmt.Errorf("--project is required when --provider=gke")
	}
	if c.Location == "" {
		return fmt.Errorf("--location is required when --provider=gke")
	}
	return nil
}
