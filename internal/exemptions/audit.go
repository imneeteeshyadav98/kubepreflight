package exemptions

type AuditClassification string

const (
	AuditGovernedExemption AuditClassification = "governed_exemption"
	AuditNonExemption      AuditClassification = "non_exemption_filter"
	AuditSeparatePath      AuditClassification = "intentionally_separate_path"
)

// AuditEntry is the QUALITY-001B inventory of reviewed suppression-like paths.
// Governed entries point at Registry IDs. Non-exemption entries document why a
// path remains ordinary rule scope, duplicate prevention, or data collection
// behavior instead of a false-positive exemption.
type AuditEntry struct {
	Path              string
	Function          string
	AffectedRules     []string
	Classification    AuditClassification
	RegistryID        string
	EvaluationPlane   Plane
	RequiredEvidence  []string
	MigrationDecision string
	Rationale         string
}

func AuditInventory() []AuditEntry {
	return []AuditEntry{
		{
			Path:              "internal/rules/api001.go",
			Function:          "isEphemeralEvent",
			AffectedRules:     []string{"API-001"},
			Classification:    AuditGovernedExemption,
			RegistryID:        API001LiveEventsID,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  MustGet(API001LiveEventsID).RequiredEvidence,
			MigrationDecision: "governed in QUALITY-001A; retained and inventoried in QUALITY-001B",
			Rationale:         "Live Event objects are ephemeral controller/client emissions, not hand-owned migration resources; manifest Events remain Blockers.",
		},
		{
			Path:              "internal/collectors/k8s/collector.go",
			Function:          "IsAutoManagedObject",
			AffectedRules:     []string{"API-001"},
			Classification:    AuditGovernedExemption,
			RegistryID:        API001AutoManagedFlowControlID,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  MustGet(API001AutoManagedFlowControlID).RequiredEvidence,
			MigrationDecision: "collector decision now references registry ID before setting AutoManaged for flowcontrol objects",
			Rationale:         "AutoManaged feeds the API-001 Info downgrade for kube-apiserver/EKS-owned flowcontrol defaults.",
		},
		{
			Path:              "internal/collectors/k8s/collector.go",
			Function:          "IsAutoManagedObject",
			AffectedRules:     []string{"API-001"},
			Classification:    AuditGovernedExemption,
			RegistryID:        API001ControllerManagedEndpointSlicesID,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  MustGet(API001ControllerManagedEndpointSlicesID).RequiredEvidence,
			MigrationDecision: "collector decision now references registry ID before setting AutoManaged for EndpointSlices",
			Rationale:         "AutoManaged feeds the API-001 Info downgrade for EndpointSlices created by the built-in EndpointSlice controller.",
		},
		{
			Path:              "internal/rules/addon001.go",
			Function:          "lookupLiveAddonCatalog",
			AffectedRules:     []string{"ADDON-001", "ADDON-002"},
			Classification:    AuditGovernedExemption,
			RegistryID:        AddonProviderScopedCatalogID,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  MustGet(AddonProviderScopedCatalogID).RequiredEvidence,
			MigrationDecision: "migrated in QUALITY-001B",
			Rationale:         "EKS-scoped compatibility facts are withheld until AWS/EKS enrichment confirms the provider plane.",
		},
		{
			Path:              "internal/rules/addon001.go",
			Function:          "classifyLiveAddon / classifyLiveAddonByImage",
			AffectedRules:     []string{"ADDON-001", "ADDON-002"},
			Classification:    AuditSeparatePath,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  []string{"strict image repository suffix for catalog-backed add-ons", "legacy ingress identity token only for always-unverifiable ADDON-002 inventory"},
			MigrationDecision: "kept outside registry as classification input, not a suppression or severity downgrade",
			Rationale:         "Classification decides whether an add-on check applies; provider-scoped compatibility borrowing is governed separately.",
		},
		{
			Path:              "internal/rules/drain001.go",
			Function:          "Evaluate / isLiveWorkloadPod",
			AffectedRules:     []string{"DRAIN-001"},
			Classification:    AuditNonExemption,
			EvaluationPlane:   PlaneBoth,
			RequiredEvidence:  []string{"Deployment or StatefulSet desired replicas effectively equal 1", "live pod must not be deleting or terminal when used as evidence"},
			MigrationDecision: "kept outside registry as rule scope and evidence hygiene",
			Rationale:         "DaemonSets, Jobs, terminal pods, and deleting pods are outside the singleton Deployment/StatefulSet availability question.",
		},
		{
			Path:              "internal/rules/drain004.go",
			Function:          "displacedDemand / hasControlPlaneTaint / podRequests",
			AffectedRules:     []string{"DRAIN-004"},
			Classification:    AuditNonExemption,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  []string{"non-DaemonSet displaced pod demand", "remaining schedulable non-control-plane node capacity", "incomplete requests counted as coverage caveat"},
			MigrationDecision: "kept outside registry as capacity model boundaries",
			Rationale:         "These paths define the aggregate drain-capacity estimate and avoid overstating spare capacity; they do not suppress a finding based on trust.",
		},
		{
			Path:              "internal/rules/drain003.go",
			Function:          "hasHostnameAntiAffinity / qualifyingNodes",
			AffectedRules:     []string{"DRAIN-003"},
			Classification:    AuditNonExemption,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  []string{"hard scheduling constraint", "current node inventory proves no spare qualifying target"},
			MigrationDecision: "kept outside registry as normal rule condition",
			Rationale:         "The rule fires only when unschedulability is proven; broader topology cases remain out of scope to avoid new false positives.",
		},
		{
			Path:              "internal/cli/scan.go internal/cli/plan.go internal/rules/node002.go internal/rules/net002.go",
			Function:          "provider/AWS graceful degradation",
			AffectedRules:     []string{"NODE-002", "NET-002", "provider enrichment"},
			Classification:    AuditNonExemption,
			EvaluationPlane:   PlaneLive,
			RequiredEvidence:  []string{"provider snapshot absent or collection error recorded"},
			MigrationDecision: "kept outside registry as coverage behavior",
			Rationale:         "A skipped provider plane is surfaced as coverage, not a trusted safe result for an affected object.",
		},
		{
			Path:              "internal/findings/filter.go",
			Function:          "FilterByNamespaces",
			AffectedRules:     []string{"all namespaced findings"},
			Classification:    AuditNonExemption,
			EvaluationPlane:   PlaneBoth,
			RequiredEvidence:  []string{"user-supplied namespace allowlist", "namespaced resource references"},
			MigrationDecision: "kept outside registry as explicit user filter",
			Rationale:         "The allowlist is a reporting scope control requested by the user, not an evidence-backed safety exemption.",
		},
	}
}
