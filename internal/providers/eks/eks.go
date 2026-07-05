// Package eks identifies the EKS provider for CLI wiring. It does not
// wrap or replace the existing, validated EKS enrichment collector
// (internal/collectors/aws) — that collection path in scan.go/plan.go is
// unaffected by this package. EKS exists here only so "eks" is a real
// providers.Provider alongside aks/gke, for symmetry and for a later
// phase's eventual rewiring of enrichment collection through a common
// interface.
package eks

// EKS identifies the EKS provider.
type EKS struct{}

// Name returns "eks".
func (EKS) Name() string { return "eks" }
