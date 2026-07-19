package exemptions

import (
	"fmt"
	"sort"
	"strings"
)

const (
	API001LiveEventsID                      = "api001-live-events"
	API001AutoManagedFlowControlID          = "api001-auto-managed-flowcontrol"
	API001ControllerManagedEndpointSlicesID = "api001-controller-managed-endpointslices"
)

type Plane string

const (
	PlaneLive     Plane = "live"
	PlaneManifest Plane = "manifest"
	PlaneBoth     Plane = "both"
)

// Entry records one accepted false-positive control. These entries are
// intentionally heavier than ordinary rule constants: an exemption changes
// how much trust a finding carries, so it must carry its own evidence,
// scope, regression coverage, and user-facing documentation.
type Entry struct {
	ID                   string
	AffectedRules        []string
	Rationale            string
	RequiredEvidence     []string
	SupportedResources   []string
	EvaluationPlane      Plane
	Behavior             string
	Scope                string
	ScopeBoundaries      []string
	ConservativeFallback string
	PositiveTests        []string
	NegativeTests        []string
	SpoofingTests        []string
	PlaneTests           []string
	Documentation        string
}

// EvidenceText returns evidence strings safe to include in a rendered
// finding. The registry keeps these strings close to the governance entry
// so implementation and documentation do not drift apart.
func (e Entry) EvidenceText() []string {
	return copyStrings(e.RequiredEvidence)
}

// Registry returns every accepted exemption/downgrade in stable order.
func Registry() []Entry {
	entries := cloneEntries(registryEntries)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries
}

var registryEntries = []Entry{
	{
		ID:              API001LiveEventsID,
		AffectedRules:   []string{"API-001"},
		Rationale:       "Live Events are emitted by clients and controllers, self-expire quickly, and do not represent a hand-owned migration object.",
		EvaluationPlane: PlaneLive,
		Behavior:        "suppressed entirely",
		Scope:           "live Kubernetes plane only; manifest Event YAML still reports as a Blocker",
		SupportedResources: []string{
			"events.k8s.io/v1beta1 Event",
		},
		RequiredEvidence: []string{
			"live Event objects are emitted by controllers/clients rather than hand-migrated by cluster operators",
			"Event objects are short-lived and can appear in high volume, so per-object Blockers are noise rather than an actionable owner signal",
			"manifest-plane Event YAML is outside this exemption and remains actionable user-authored input",
		},
		ScopeBoundaries: []string{
			"does not apply to manifest-plane Event YAML",
			"does not apply to non-Event deprecated API objects",
		},
		ConservativeFallback: "Report the deprecated object normally when it is not a live Event.",
		PositiveTests:        []string{"TestAPI001_LiveEvents_SuppressedEntirely", "TestAPI002_LiveEventsSuppressed"},
		NegativeTests:        []string{"TestAPI001_ManifestEvents_StillFireAsBlocker"},
		SpoofingTests:        []string{"TestAPI001_DemoSeededObjects_UnaffectedByEphemeralFiltering"},
		PlaneTests:           []string{"TestAPI001_ManifestEvents_StillFireAsBlocker"},
		Documentation:        "docs/false-positive-governance.md#api-001-live-events",
	},
	{
		ID:              API001AutoManagedFlowControlID,
		AffectedRules:   []string{"API-001"},
		Rationale:       "Kube-apiserver and EKS reconcile their own flowcontrol defaults, so these live objects are inventory signal rather than an operator-owned migration task.",
		EvaluationPlane: PlaneLive,
		Behavior:        "downgraded from Blocker to Info",
		Scope:           "live flowcontrol.apiserver.k8s.io objects only when controller evidence is present",
		SupportedResources: []string{
			"flowcontrol.apiserver.k8s.io FlowSchema",
			"flowcontrol.apiserver.k8s.io PriorityLevelConfiguration",
		},
		RequiredEvidence: []string{
			"reconciled automatically: matched via the apf.kubernetes.io/autoupdate-spec annotation (kube-apiserver bootstrap default) or EKS's eks-internal field manager (cloud-provider-injected default)",
			"GVK guard: the eks-internal field manager is only trusted for flowcontrol.apiserver.k8s.io FlowSchema/PriorityLevelConfiguration objects",
			"name alone is never trusted; an object merely named like an EKS default remains a Blocker without controller evidence",
			"manifest-plane FlowSchema/PriorityLevelConfiguration YAML is outside this exemption and remains a Blocker",
		},
		ScopeBoundaries: []string{
			"does not apply to manifest-plane FlowSchema/PriorityLevelConfiguration YAML",
			"does not apply to flowcontrol objects without auto-managed evidence",
			"does not apply to non-flowcontrol objects with matching names, annotations, labels, or field managers",
		},
		ConservativeFallback: "Report as a Blocker when auto-managed evidence is missing, ambiguous, or attached to an unsupported GVK.",
		PositiveTests:        []string{"TestAPI001_AutoManagedFlowSchema_DowngradesToInfo", "TestIsAutoManagedObject/EKS-injected_FlowSchema_has_autoupdate-spec_false_but_the_eks-internal_field_manager"},
		NegativeTests:        []string{"TestAPI001_UserCreatedFlowSchema_StillFiresAsBlocker"},
		SpoofingTests:        []string{"TestAPI001_FlowControlNameOnlySpoofing_StillFiresAsBlocker", "TestIsAutoManagedObject/eks-internal_field_manager_on_a_non-flowcontrol_GVK_grants_no_exemption"},
		PlaneTests:           []string{"TestAPI001_ManifestPlaneFlowSchema_AlwaysBlocker"},
		Documentation:        "docs/false-positive-governance.md#api-001-auto-managed-flowcontrol",
	},
	{
		ID:              API001ControllerManagedEndpointSlicesID,
		AffectedRules:   []string{"API-001"},
		Rationale:       "The built-in EndpointSlice controller recreates its own slices against the owning Service at the API version the server currently serves.",
		EvaluationPlane: PlaneLive,
		Behavior:        "downgraded from Blocker to Info",
		Scope:           "live EndpointSlice objects only when the built-in EndpointSlice controller managed-by label is present",
		SupportedResources: []string{
			"discovery.k8s.io/v1beta1 EndpointSlice",
		},
		RequiredEvidence: []string{
			"endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io (controller-owned, recreated automatically)",
			"namespace/name alone is never trusted; default/kubernetes-shaped EndpointSlices remain Blockers without controller evidence",
			"manifest-plane EndpointSlice YAML is outside this exemption and remains a Blocker",
		},
		ScopeBoundaries: []string{
			"does not apply to manifest-plane EndpointSlice YAML",
			"does not apply to EndpointSlices missing the controller managed-by label",
			"does not apply to non-EndpointSlice objects with similar labels, namespaces, or names",
		},
		ConservativeFallback: "Report as a Blocker when controller-managed evidence is missing, ambiguous, or attached to an unsupported GVK.",
		PositiveTests:        []string{"TestAPI001_ControllerManagedEndpointSlice_DowngradesToInfo", "TestIsAutoManagedObject/controller-managed_EndpointSlice_carries_the_real_managed-by_label"},
		NegativeTests:        []string{"TestAPI001_UnmanagedEndpointSlice_StillFiresAsBlocker"},
		SpoofingTests:        []string{"TestAPI001_EndpointSliceNamespaceOnlySpoofing_StillFiresAsBlocker", "TestIsAutoManagedObject/the_default/kubernetes_EndpointSlice_exception_has_no_managed-by_label"},
		PlaneTests:           []string{"TestAPI001_ManifestEndpointSlice_UnaffectedByAutoManagedCheck"},
		Documentation:        "docs/false-positive-governance.md#api-001-controller-managed-endpointslices",
	},
}

// MustGet returns an entry by ID and panics if the source registry is
// inconsistent. Rule code calls this for entries that are required at
// runtime, while tests validate the whole registry.
func MustGet(id string) Entry {
	for _, entry := range registryEntries {
		if entry.ID == id {
			return cloneEntry(entry)
		}
	}
	panic(fmt.Sprintf("unknown exemption registry entry %q", id))
}

// Validate returns governance failures for incomplete or duplicate entries.
func Validate(entries []Entry) []error {
	var errs []error
	seen := map[string]bool{}
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			errs = append(errs, fmt.Errorf("entry with rationale %q has empty ID", entry.Rationale))
			continue
		}
		if seen[entry.ID] {
			errs = append(errs, fmt.Errorf("%s: duplicate ID", entry.ID))
		}
		seen[entry.ID] = true
		if strings.TrimSpace(entry.Rationale) == "" {
			errs = append(errs, fmt.Errorf("%s: missing Rationale", entry.ID))
		}
		if !validPlane(entry.EvaluationPlane) {
			errs = append(errs, fmt.Errorf("%s: invalid EvaluationPlane %q", entry.ID, entry.EvaluationPlane))
		}
		requireList(&errs, entry.ID, "AffectedRules", entry.AffectedRules)
		requireList(&errs, entry.ID, "RequiredEvidence", entry.RequiredEvidence)
		requireList(&errs, entry.ID, "SupportedResources", entry.SupportedResources)
		requireList(&errs, entry.ID, "ScopeBoundaries", entry.ScopeBoundaries)
		if strings.TrimSpace(entry.Behavior) == "" {
			errs = append(errs, fmt.Errorf("%s: missing Behavior", entry.ID))
		}
		if strings.TrimSpace(entry.Scope) == "" {
			errs = append(errs, fmt.Errorf("%s: missing Scope", entry.ID))
		}
		if strings.TrimSpace(entry.Documentation) == "" {
			errs = append(errs, fmt.Errorf("%s: missing Documentation", entry.ID))
		}
		if strings.TrimSpace(entry.ConservativeFallback) == "" {
			errs = append(errs, fmt.Errorf("%s: missing ConservativeFallback", entry.ID))
		}
		requireList(&errs, entry.ID, "PositiveTests", entry.PositiveTests)
		requireList(&errs, entry.ID, "NegativeTests", entry.NegativeTests)
		requireList(&errs, entry.ID, "SpoofingTests", entry.SpoofingTests)
		requireList(&errs, entry.ID, "PlaneTests", entry.PlaneTests)
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

func requireList(errs *[]error, id, field string, values []string) {
	if len(values) == 0 {
		*errs = append(*errs, fmt.Errorf("%s: missing %s", id, field))
		return
	}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			*errs = append(*errs, fmt.Errorf("%s: %s[%d] is empty", id, field, i))
		}
	}
}

func validPlane(plane Plane) bool {
	switch plane {
	case PlaneLive, PlaneManifest, PlaneBoth:
		return true
	default:
		return false
	}
}

func cloneEntries(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	for i, entry := range entries {
		out[i] = cloneEntry(entry)
	}
	return out
}

func cloneEntry(entry Entry) Entry {
	entry.AffectedRules = copyStrings(entry.AffectedRules)
	entry.RequiredEvidence = copyStrings(entry.RequiredEvidence)
	entry.SupportedResources = copyStrings(entry.SupportedResources)
	entry.ScopeBoundaries = copyStrings(entry.ScopeBoundaries)
	entry.PositiveTests = copyStrings(entry.PositiveTests)
	entry.NegativeTests = copyStrings(entry.NegativeTests)
	entry.SpoofingTests = copyStrings(entry.SpoofingTests)
	entry.PlaneTests = copyStrings(entry.PlaneTests)
	return entry
}

func copyStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	return out
}
