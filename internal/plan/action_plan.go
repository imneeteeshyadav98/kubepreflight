package plan

import (
	"sort"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

const ActionPlanSchemaVersion = "kubepreflight.io/upgrade-action-plan/v1"

const (
	ActionStatusRequired    = "required"
	ActionStatusRecommended = "recommended"
	ActionStatusBlocked     = "blocked"
	ActionStatusReady       = "ready"
	ActionStatusManual      = "manual"
)

// UpgradeActionPlan is a structured, operator-facing checklist for taking a
// cluster from findings to a controlled Kubernetes upgrade.
type UpgradeActionPlan struct {
	SchemaVersion string        `json:"schemaVersion"`
	Verdict       string        `json:"verdict"`
	GeneratedAt   time.Time     `json:"generatedAt"`
	Phases        []ActionPhase `json:"phases"`
}

type ActionPhase struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Gate        string       `json:"gate,omitempty"`
	Actions     []PlanAction `json:"actions"`
}

type PlanAction struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Required        bool     `json:"required"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason,omitempty"`
	SourceRuleIDs   []string `json:"sourceRuleIds,omitempty"`
	SuccessCriteria []string `json:"successCriteria,omitempty"`
	Commands        []string `json:"commands,omitempty"`
}

// BuildActionPlan derives a stable four-phase upgrade checklist from the
// immediate next-hop findings. It never fabricates findings: Phase 1 and any
// sourceRuleIds are present only when matching rules actually appeared.
//
// Defensively refuses to build a normal plan when r represents a downgrade
// (see isDowngrade) -- the CLI layer (internal/cli's rejectDowngrade)
// already fails a downgrade scan/plan fast before this is ever reached in
// practice, but a four-phase "upgrade preparation / upgrade execution"
// checklist would be actively misleading for a version transition going
// the other direction, so this is a second, independent guard: no future
// caller of BuildActionPlan (a new command, a test, a library consumer)
// can accidentally produce one.
func BuildActionPlan(r *findings.Report, now time.Time) *UpgradeActionPlan {
	if isDowngrade(r) {
		return &UpgradeActionPlan{
			SchemaVersion: ActionPlanSchemaVersion,
			Verdict:       "DOWNGRADE_NOT_SUPPORTED",
			GeneratedAt:   now,
		}
	}

	phase1Actions := criticalBlockerActions(r)
	blocked := hasBlockingPhase1Action(phase1Actions)

	return &UpgradeActionPlan{
		SchemaVersion: ActionPlanSchemaVersion,
		Verdict:       actionPlanVerdict(r, blocked),
		GeneratedAt:   now,
		Phases: []ActionPhase{
			{
				ID:          "phase-1-critical-blockers",
				Title:       "Phase 1 - Critical Blockers",
				Description: "Fix findings that can prevent a safe upgrade or make upgrade remediation fail.",
				Gate:        "Phase 3 is blocked until every required action in this phase is resolved.",
				Actions:     phase1Actions,
			},
			upgradePreparationPhase(r),
			upgradeExecutionPhase(blocked),
			validationPhase(),
		},
	}
}

// isDowngrade reports whether r represents a confidently-known downgrade
// (target below current, same major) -- false whenever CurrentVersion is
// empty or either version fails to parse, matching the "don't guess"
// principle applied throughout this codebase (e.g.
// findings.Report.UpgradeApplicable's same-version case): a downgrade
// must never be assumed without being sure.
func isDowngrade(r *findings.Report) bool {
	if r == nil || r.CurrentVersion == "" {
		return false
	}
	relation, err := findings.CompareMinorVersions(r.CurrentVersion, r.TargetVersion)
	return err == nil && relation == findings.VersionDowngrade
}

func actionPlanVerdict(r *findings.Report, blocked bool) string {
	if r != nil && !r.IsComplete() {
		return "ASSESSMENT_INCOMPLETE"
	}
	if blocked {
		return "BLOCKED"
	}
	return "READY"
}

type actionTemplate struct {
	id                       string
	title                    string
	sourceRuleIDs            []string
	optionalWhenOnlyWarnings bool
	successCriteria          []string
	commands                 []string
}

func criticalBlockerActions(r *findings.Report) []PlanAction {
	templates := []actionTemplate{
		{
			id:                       "fix-api-compatibility",
			title:                    "Fix removed or deprecated API usage",
			sourceRuleIDs:            []string{"API-001", "API-002", "CRD-001", "CRD-002", "APISERVICE-001"},
			optionalWhenOnlyWarnings: true,
			successCriteria: []string{
				"All manifests and live resources use APIs served by the target Kubernetes version.",
				"Re-run KubePreflight and confirm no API compatibility findings remain.",
			},
		},
		{
			id:            "fix-eks-critical-insights",
			title:         "Resolve EKS critical upgrade insights",
			sourceRuleIDs: []string{"EKS-INSIGHT-001"},
			successCriteria: []string{
				"AWS EKS Upgrade Insights no longer reports ERROR upgrade-readiness findings.",
				"Re-run KubePreflight with AWS enrichment enabled.",
			},
		},
		{
			id:            "fix-fail-closed-webhooks",
			title:         "Fix fail-closed admission webhooks",
			sourceRuleIDs: []string{"WH-001", "WH-002", "WH-004", "WH-005"},
			successCriteria: []string{
				"Admission webhooks have healthy endpoints and scoped rules.",
				"kubectl apply, patch, scale, and Helm operations are not blocked by webhook failures.",
			},
			commands: []string{
				"kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations",
				"kubectl get endpointslices,endpoints --all-namespaces",
			},
		},
		{
			id:            "resolve-disruption-risk",
			title:         "Resolve disruption budget and unhealthy workload risks",
			sourceRuleIDs: []string{"PDB-001", "PDB-002", "DRAIN-001"},
			successCriteria: []string{
				"Workloads protected by PodDisruptionBudgets can tolerate at least one voluntary disruption.",
				"Unhealthy workloads are repaired before node drains or managed node group upgrades.",
				"Single-replica Deployments/StatefulSets have real eviction headroom or an explicitly documented waiver.",
			},
			commands: []string{
				"kubectl get pdb --all-namespaces",
				"kubectl get pods --all-namespaces",
			},
		},
		{
			id:                       "resolve-node-local-storage-risk",
			title:                    "Resolve node-local storage evacuation risk",
			sourceRuleIDs:            []string{"DRAIN-002"},
			optionalWhenOnlyWarnings: true,
			successCriteria: []string{
				"Workloads using hostPath or node-pinned PersistentVolumes have a documented drain/migration plan, or have moved to networked storage.",
			},
			commands: []string{
				"kubectl get pv -o wide",
				"kubectl get pvc --all-namespaces",
			},
		},
		{
			id:                       "resolve-scheduling-constraint-risk",
			title:                    "Resolve hard scheduling constraint risk",
			sourceRuleIDs:            []string{"DRAIN-003"},
			optionalWhenOnlyWarnings: true,
			successCriteria: []string{
				"Node affinity/selector, anti-affinity, topology spread, and hostPort constraints have enough qualifying nodes/domains to survive a node drain.",
			},
			commands: []string{
				"kubectl get nodes --show-labels",
				"kubectl describe node <node>",
			},
		},
		{
			id:                       "resolve-unhealthy-workloads",
			title:                    "Resolve unhealthy workloads before upgrade",
			sourceRuleIDs:            []string{"WORKLOAD-001"},
			optionalWhenOnlyWarnings: true,
			successCriteria: []string{
				"Pre-existing unhealthy workloads are fixed or explicitly waived before the change window.",
				"Post-upgrade validation owners know which workload health issues predated the upgrade.",
			},
			commands: []string{
				"kubectl get pods --all-namespaces",
				"kubectl get events --all-namespaces --sort-by=.lastTimestamp",
			},
		},
		{
			id:                       "resolve-statefulset-daemonset-readiness",
			title:                    "Resolve StatefulSet/DaemonSet rollout health",
			sourceRuleIDs:            []string{"DRAIN-005"},
			optionalWhenOnlyWarnings: true,
			successCriteria: []string{
				"StatefulSets and DaemonSets have all desired replicas/pods Ready before starting or continuing a node drain.",
			},
			commands: []string{
				"kubectl get statefulsets,daemonsets --all-namespaces",
				"kubectl get pods --all-namespaces -o wide",
			},
		},
		{
			id:                       "replace-deprecated-master-node-label",
			title:                    "Replace deprecated master node label selectors",
			sourceRuleIDs:            []string{"NODE-003"},
			optionalWhenOnlyWarnings: true,
			successCriteria: []string{
				"No live or manifest pod templates reference node-role.kubernetes.io/master in node selectors, node affinity, or tolerations.",
				"Target nodes carry the replacement node-role.kubernetes.io/control-plane label or an approved platform-owned label before selectors are changed.",
			},
			commands: []string{
				"kubectl get nodes --show-labels | grep -E 'node-role.kubernetes.io/(master|control-plane)'",
				"kubectl get deploy,daemonset --all-namespaces -o yaml",
			},
		},
		{
			id:            "resolve-not-ready-nodes",
			title:         "Resolve NotReady nodes",
			sourceRuleIDs: []string{"NODE-001"},
			successCriteria: []string{
				"All nodes are Ready before starting the upgrade.",
				"Node drain and scheduling capacity are healthy.",
			},
			commands: []string{"kubectl get nodes"},
		},
		{
			id:                       "resolve-node-capacity-risk",
			title:                    "Resolve estimated node-capacity shortage",
			sourceRuleIDs:            []string{"DRAIN-004"},
			optionalWhenOnlyWarnings: true,
			successCriteria: []string{
				"Remaining nodes have enough estimated spare CPU/memory to absorb any one node's workloads if it's removed, or a cluster autoscaler is confirmed to add capacity during drains.",
			},
			commands: []string{
				"kubectl describe nodes | grep -A5 'Allocated resources'",
				"kubectl top nodes",
			},
		},
		{
			id:            "fix-coredns-health",
			title:         "Fix CoreDNS health",
			sourceRuleIDs: []string{"COREDNS-001"},
			successCriteria: []string{
				"CoreDNS pods are available and ready.",
				"DNS lookups from workloads succeed before the upgrade window.",
			},
			commands: []string{
				"kubectl -n kube-system get deploy,pods -l k8s-app=kube-dns",
			},
		},
	}

	actions := make([]PlanAction, 0, len(templates))
	for _, tmpl := range templates {
		sourceRuleIDs := presentRuleIDs(r, tmpl.sourceRuleIDs...)
		if len(sourceRuleIDs) == 0 {
			continue
		}
		required := true
		status := ActionStatusRequired
		reason := "Required because matching findings were detected in the current assessment."
		if tmpl.optionalWhenOnlyWarnings && !hasBlockerForRules(r, sourceRuleIDs...) {
			required = false
			status = ActionStatusRecommended
			reason = "Recommended because matching warning findings were detected in the current assessment."
		}
		actions = append(actions, PlanAction{
			ID:              tmpl.id,
			Title:           tmpl.title,
			Required:        required,
			Status:          status,
			Reason:          reason,
			SourceRuleIDs:   sourceRuleIDs,
			SuccessCriteria: tmpl.successCriteria,
			Commands:        tmpl.commands,
		})
	}
	return actions
}

func upgradePreparationPhase(r *findings.Report) ActionPhase {
	return ActionPhase{
		ID:          "phase-2-upgrade-preparation",
		Title:       "Phase 2 - Upgrade Preparation",
		Description: "Confirm the operational prerequisites for a controlled upgrade window.",
		Gate:        "Proceed only after the change owner confirms every required preparation action.",
		Actions: []PlanAction{
			{
				ID:              "verify-backups",
				Title:           "Verify backups and control-plane recovery strategy",
				Required:        true,
				Status:          ActionStatusManual,
				SuccessCriteria: []string{"Backup and recovery ownership is documented in the change ticket."},
			},
			{
				ID:              "confirm-rollback-plan",
				Title:           "Confirm rollback and abort plan",
				Required:        true,
				Status:          ActionStatusManual,
				SuccessCriteria: []string{"Rollback triggers and abort criteria are documented before the window starts."},
			},
			{
				ID:              "check-node-readiness",
				Title:           "Check node readiness",
				Required:        true,
				Status:          ActionStatusManual,
				SourceRuleIDs:   presentRuleIDs(r, "NODE-001", "NODE-002", "EKS-NG-001", "EKS-NG-002", "EKS-NG-003", "EKS-NG-004"),
				SuccessCriteria: []string{"Nodes and managed node groups are healthy enough to drain and replace safely."},
				Commands:        []string{"kubectl get nodes"},
			},
			{
				ID:              "validate-addon-compatibility",
				Title:           "Validate add-on compatibility",
				Required:        true,
				Status:          ActionStatusManual,
				SourceRuleIDs:   presentRuleIDs(r, "ADDON-001", "ADDON-002", "COREDNS-001", "EKS-INSIGHT-002", "EKS-INSIGHT-003"),
				SuccessCriteria: []string{"Cluster add-ons are compatible with the target Kubernetes version."},
			},
			{
				ID:              "confirm-maintenance-window",
				Title:           "Confirm maintenance window",
				Required:        true,
				Status:          ActionStatusManual,
				SuccessCriteria: []string{"The window covers control-plane, node, add-on, and validation work."},
			},
			{
				ID:              "confirm-workload-owner-approval",
				Title:           "Confirm workload owner approval",
				Required:        true,
				Status:          ActionStatusManual,
				SuccessCriteria: []string{"Application owners acknowledge expected disruption risk and validation responsibilities."},
			},
		},
	}
}

func upgradeExecutionPhase(blocked bool) ActionPhase {
	status := ActionStatusReady
	reason := ""
	if blocked {
		status = ActionStatusBlocked
		reason = "Blocked until critical upgrade blockers are resolved."
	}

	actions := []PlanAction{
		{
			ID:              "upgrade-control-plane",
			Title:           "Upgrade control plane",
			Required:        true,
			Status:          status,
			Reason:          reason,
			SuccessCriteria: []string{"Control plane reports the target Kubernetes version and API health is normal."},
		},
		{
			ID:              "upgrade-worker-nodes",
			Title:           "Upgrade managed node groups or worker nodes",
			Required:        true,
			Status:          status,
			Reason:          reason,
			SuccessCriteria: []string{"All worker nodes are on supported kubelet versions and Ready."},
		},
		{
			ID:              "upgrade-addons",
			Title:           "Upgrade cluster add-ons",
			Required:        true,
			Status:          status,
			Reason:          reason,
			SuccessCriteria: []string{"CoreDNS, kube-proxy, CNI, and provider-managed add-ons are healthy."},
		},
		{
			ID:              "restart-workloads-if-required",
			Title:           "Restart or roll workloads if required",
			Required:        false,
			Status:          status,
			Reason:          reason,
			SuccessCriteria: []string{"Workloads affected by node replacement or add-on changes are available."},
		},
	}

	return ActionPhase{
		ID:          "phase-3-upgrade",
		Title:       "Phase 3 - Upgrade",
		Description: "Execute the control-plane, node, and add-on upgrade steps.",
		Gate:        "Do not start this phase while any action is blocked.",
		Actions:     actions,
	}
}

func validationPhase() ActionPhase {
	return ActionPhase{
		ID:          "phase-4-validation",
		Title:       "Phase 4 - Validation",
		Description: "Prove cluster and application health after the upgrade.",
		Gate:        "Close the change only after validation owners confirm these checks.",
		Actions: []PlanAction{
			{ID: "validate-dns", Title: "Validate DNS", Required: true, Status: ActionStatusManual, SuccessCriteria: []string{"Service discovery and external DNS paths resolve successfully."}},
			{ID: "validate-ingress", Title: "Validate ingress", Required: true, Status: ActionStatusManual, SuccessCriteria: []string{"Ingress, gateway, and load balancer routes serve expected traffic."}},
			{ID: "validate-workload-health", Title: "Validate workload health", Required: true, Status: ActionStatusManual, SuccessCriteria: []string{"Deployments, StatefulSets, DaemonSets, and critical pods are healthy."}},
			{ID: "validate-metrics", Title: "Validate metrics", Required: true, Status: ActionStatusManual, SuccessCriteria: []string{"Metrics pipelines show normal scrape and ingestion behavior."}},
			{ID: "validate-logs", Title: "Validate logs", Required: true, Status: ActionStatusManual, SuccessCriteria: []string{"Application and platform logs show no upgrade-related error spike."}},
			{ID: "run-smoke-tests", Title: "Run smoke tests", Required: true, Status: ActionStatusManual, SuccessCriteria: []string{"Business-critical smoke tests pass after the upgrade."}},
		},
	}
}

func presentRuleIDs(r *findings.Report, ruleIDs ...string) []string {
	if r == nil || len(ruleIDs) == 0 {
		return nil
	}

	wanted := make(map[string]struct{}, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		wanted[ruleID] = struct{}{}
	}

	seen := make(map[string]struct{})
	for _, f := range r.Findings {
		if _, ok := wanted[f.RuleID]; ok {
			seen[f.RuleID] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for ruleID := range seen {
		out = append(out, ruleID)
	}
	sort.Strings(out)
	return out
}

func hasBlockerForRules(r *findings.Report, ruleIDs ...string) bool {
	if r == nil {
		return false
	}
	wanted := make(map[string]struct{}, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		wanted[ruleID] = struct{}{}
	}
	for _, f := range r.Findings {
		if _, ok := wanted[f.RuleID]; ok && f.Severity == findings.SeverityBlocker {
			return true
		}
	}
	return false
}

func hasBlockingPhase1Action(actions []PlanAction) bool {
	for _, action := range actions {
		if action.Required && action.Status == ActionStatusRequired {
			return true
		}
	}
	return false
}
