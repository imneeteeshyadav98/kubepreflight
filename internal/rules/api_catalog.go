package rules

import (
	"fmt"

	"kubepreflight/internal/apicatalog"
)

// apiRemovalDecision is the removed-version fact API-001/API-002 evaluate
// a target version against, always sourced from the reviewed
// apicatalog.VersionedCatalog. apicatalog.Deprecated — the list the k8s
// and manifest collectors use to discover live/manifest objects at
// deprecated GVKs in the first place — is itself derived from this same
// versioned catalog (see apicatalog.legacyFromVersioned), so every object
// either rule ever sees is guaranteed to have a matching catalog entry;
// there is no legacy fallback path left to fall back to.
type apiRemovalDecision struct {
	RemovedInVersion string
	Source           string
	Reference        string
}

// resolveAPIRemoval looks up group/version/kind in the reviewed
// apicatalog.VersionedCatalog via EntryFor (which, unlike Lookup, isn't
// gated by any target-version range — API-001/API-002 need to know
// whether this GVK is known to the catalog at all, then compare its
// RemovedInVersion against the target themselves).
//
// A miss here is a catalog integrity error, not an "unknown API" the
// rule should quietly skip: apicatalog.Deprecated is derived from this
// exact catalog, so a live or manifest object reported at some
// group/version/kind that the catalog itself doesn't have an entry for
// can only mean the two have drifted out of sync — a bug to surface
// loudly, never a silent guess.
func resolveAPIRemoval(group, version, kind string) (apiRemovalDecision, error) {
	vc, err := apicatalog.DefaultVersioned()
	if err != nil {
		return apiRemovalDecision{}, fmt.Errorf("loading API version catalog: %w", err)
	}
	entry, ok := vc.EntryFor(group, version, kind)
	if !ok {
		return apiRemovalDecision{}, fmt.Errorf(
			"API version catalog integrity error: no entry for %s/%s %s, but it was reported as a known deprecated API by the collector — the versioned catalog and its derived legacy inventory have drifted out of sync",
			group, version, kind)
	}
	return apiRemovalDecision{
		RemovedInVersion: entry.RemovedInVersion,
		Source:           entry.Source,
		Reference:        entry.Reference,
	}, nil
}

// evidence returns the catalog provenance evidence lines every
// catalog-backed decision carries.
func (d apiRemovalDecision) evidence() []string {
	return []string{
		"catalog source: " + d.Source,
		"catalog reference: " + d.Reference,
	}
}
