package v1compat

import (
	"fmt"
	"sort"
	"strings"
)

const (
	StableScanSchemaVersion       = "1.0"
	StableComparisonSchemaVersion = "kubepreflight.io/scan-comparison/v1"
	StableActionPlanSchemaVersion = "kubepreflight.io/upgrade-action-plan/v1"
	RollbackSchemaVersion         = "kubepreflight.io/rollback-assessment/v1alpha1"
	SupportedTargetMinimum        = "1.25"
	SupportedTargetMaximum        = "1.39"
	FingerprintDomain             = "finding-v2"
)

type Flag struct {
	Name     string
	Default  string
	Required bool
}

type Command struct {
	Path    string
	Aliases []string
	Flags   []Flag
}

type Issue struct {
	Message string
}

func (i Issue) Error() string { return i.Message }

type Report struct {
	CommandCount int
	RuleCount    int
	Issues       []Issue
}

func (r Report) OK() bool { return len(r.Issues) == 0 }

func (r Report) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "V1 compatibility contract: %d command(s), %d rule ID(s)\n", r.CommandCount, r.RuleCount)
	if r.OK() {
		sb.WriteString("Compatibility contract check: OK\n")
		return sb.String()
	}
	sb.WriteString("Compatibility contract check: FAILED\n")
	for _, issue := range r.Issues {
		fmt.Fprintf(&sb, "  - %s\n", issue.Message)
	}
	return sb.String()
}

type Actual struct {
	Commands                  []Command
	SchemaVersions            map[string]string
	RuleIDs                   []string
	DefaultPriorities         map[string]string
	FingerprintV2Sample       string
	IncompleteResult          string
	IncompleteExitCode        int
	BlockerResult             string
	BlockerExitCode           int
	WarningResult             string
	WarningExitCode           int
	CleanResult               string
	CleanExitCode             int
	InfraFailureExitCode      int
	GenericErrorExitCode      int
	CompareGateFailExitCode   int
	RollbackPreferredExit     int
	RollbackDoNotProceedExit  int
	RollbackNeedsOperatorExit int
}

func Check(actual Actual) Report {
	report := Report{CommandCount: len(actual.Commands), RuleCount: len(actual.RuleIDs)}
	report.add(checkCommands(actual.Commands)...)
	report.add(checkSchemaVersions(actual.SchemaVersions)...)
	report.add(checkRuleIDs(actual.RuleIDs)...)
	report.add(checkPriorities(actual.DefaultPriorities)...)
	report.add(checkExitCodes(actual)...)
	report.add(checkFingerprint(actual.FingerprintV2Sample)...)
	sort.Slice(report.Issues, func(i, j int) bool {
		return report.Issues[i].Message < report.Issues[j].Message
	})
	return report
}

func (r *Report) add(issues ...Issue) {
	r.Issues = append(r.Issues, issues...)
}

func checkCommands(actual []Command) []Issue {
	got := map[string]Command{}
	for _, command := range actual {
		got[command.Path] = command
	}
	var issues []Issue
	for _, want := range ExpectedCommands() {
		actualCommand, ok := got[want.Path]
		if !ok {
			issues = append(issues, Issue{Message: fmt.Sprintf("missing command %q", want.Path)})
			continue
		}
		if strings.Join(actualCommand.Aliases, "\x00") != strings.Join(want.Aliases, "\x00") {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s aliases = %v, want %v", want.Path, actualCommand.Aliases, want.Aliases)})
		}
		issues = append(issues, checkFlags(want.Path, actualCommand.Flags, want.Flags)...)
	}
	for path := range got {
		if !hasCommand(path) {
			issues = append(issues, Issue{Message: fmt.Sprintf("unexpected command %q", path)})
		}
	}
	return issues
}

func checkFlags(path string, actual, expected []Flag) []Issue {
	got := map[string]Flag{}
	for _, flag := range actual {
		got[flag.Name] = flag
	}
	var issues []Issue
	for _, want := range expected {
		actualFlag, ok := got[want.Name]
		if !ok {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s missing --%s", path, want.Name)})
			continue
		}
		if actualFlag.Default != want.Default {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s --%s default = %q, want %q", path, want.Name, actualFlag.Default, want.Default)})
		}
		if actualFlag.Required != want.Required {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s --%s required = %v, want %v", path, want.Name, actualFlag.Required, want.Required)})
		}
	}
	for name := range got {
		if !hasFlag(expected, name) {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s has unexpected flag --%s", path, name)})
		}
	}
	return issues
}

func checkSchemaVersions(actual map[string]string) []Issue {
	expected := map[string]string{
		"findings":         StableScanSchemaVersion,
		"plan":             StableScanSchemaVersion,
		"actionPlan":       StableActionPlanSchemaVersion,
		"comparison":       StableComparisonSchemaVersion,
		"rollbackExcluded": RollbackSchemaVersion,
		"apiCatalog":       "apicatalog.kubepreflight.io/v1",
		"compatCatalog":    "compatcatalog.kubepreflight.io/v1",
	}
	var issues []Issue
	for name, want := range expected {
		if got := actual[name]; got != want {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s schemaVersion = %q, want %q", name, got, want)})
		}
	}
	return issues
}

func checkRuleIDs(actual []string) []Issue {
	return compareStringList("registered rule IDs", actual, ExpectedRuleIDs())
}

func checkPriorities(actual map[string]string) []Issue {
	var issues []Issue
	for ruleID, want := range ExpectedDefaultPriorities() {
		if got := actual[ruleID]; got != want {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s default priority = %q, want %q", ruleID, got, want)})
		}
	}
	for ruleID := range actual {
		if _, ok := ExpectedDefaultPriorities()[ruleID]; !ok {
			issues = append(issues, Issue{Message: fmt.Sprintf("unexpected priority mapping for %s", ruleID)})
		}
	}
	return issues
}

func checkExitCodes(actual Actual) []Issue {
	expected := map[string]struct {
		result string
		code   int
	}{
		"clean":                 {result: "CLEAN", code: 0},
		"warnings":              {result: "PASSED_WITH_WARNINGS", code: 1},
		"blockers":              {result: "BLOCKED", code: 2},
		"incomplete":            {result: "INCOMPLETE", code: 3},
		"infraFailure":          {code: 4},
		"genericError":          {code: 1},
		"compareGateFail":       {code: 1},
		"rollbackPreferred":     {code: 0},
		"rollbackDoNotProceed":  {code: 2},
		"rollbackNeedsOperator": {code: 1},
	}
	actuals := map[string]struct {
		result string
		code   int
	}{
		"clean":                 {actual.CleanResult, actual.CleanExitCode},
		"warnings":              {actual.WarningResult, actual.WarningExitCode},
		"blockers":              {actual.BlockerResult, actual.BlockerExitCode},
		"incomplete":            {actual.IncompleteResult, actual.IncompleteExitCode},
		"infraFailure":          {code: actual.InfraFailureExitCode},
		"genericError":          {code: actual.GenericErrorExitCode},
		"compareGateFail":       {code: actual.CompareGateFailExitCode},
		"rollbackPreferred":     {code: actual.RollbackPreferredExit},
		"rollbackDoNotProceed":  {code: actual.RollbackDoNotProceedExit},
		"rollbackNeedsOperator": {code: actual.RollbackNeedsOperatorExit},
	}
	var issues []Issue
	for name, want := range expected {
		got := actuals[name]
		if want.result != "" && got.result != want.result {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s result = %q, want %q", name, got.result, want.result)})
		}
		if got.code != want.code {
			issues = append(issues, Issue{Message: fmt.Sprintf("%s exit code = %d, want %d", name, got.code, want.code)})
		}
	}
	return issues
}

func checkFingerprint(actual string) []Issue {
	const want = "82cbaec03e4fd838b1ce5b9eda1c4d297f0bc05db73c0632b379813912bb8a40"
	if actual != want {
		return []Issue{{Message: fmt.Sprintf("FingerprintV2 sample = %q, want %q", actual, want)}}
	}
	return nil
}

func compareStringList(label string, actual, expected []string) []Issue {
	if strings.Join(actual, "\x00") == strings.Join(expected, "\x00") {
		return nil
	}
	return []Issue{{Message: fmt.Sprintf("%s = %v, want %v", label, actual, expected)}}
}

func hasCommand(path string) bool {
	for _, command := range ExpectedCommands() {
		if command.Path == path {
			return true
		}
	}
	return false
}

func hasFlag(flags []Flag, name string) bool {
	for _, flag := range flags {
		if flag.Name == name {
			return true
		}
	}
	return false
}

func ExpectedCommands() []Command {
	return []Command{
		{Path: "kubepreflight"},
		{Path: "kubepreflight compare", Flags: []Flag{
			{Name: "baseline", Required: true},
			{Name: "current", Required: true},
			{Name: "fail-on-new-blockers", Default: "true"},
			{Name: "fail-on-verdict-regression", Default: "true"},
			{Name: "gate-out"},
			{Name: "json-out"},
			{Name: "markdown-out"},
			{Name: "minimum-score-delta", Default: "0"},
			{Name: "redact-sensitive-identifiers", Default: "false"},
			{Name: "warning-policy", Default: "ignore"},
		}},
		{Path: "kubepreflight plan", Flags: []Flag{
			{Name: "action-plan-md"},
			{Name: "action-plan-out"},
			{Name: "allow-remote-report", Default: "false"},
			{Name: "cluster-name"},
			{Name: "collector-concurrency", Default: "4"},
			{Name: "collector-timeout", Default: "30s"},
			{Name: "context"},
			{Name: "findings-out", Default: "findings.json"},
			{Name: "from-version", Default: "auto"},
			{Name: "helm-chart", Default: "[]"},
			{Name: "kubeconfig"},
			{Name: "listen", Default: "127.0.0.1:8080"},
			{Name: "location"},
			{Name: "manifests", Default: "[]"},
			{Name: "namespace-allowlist", Default: "[]"},
			{Name: "open-report", Default: "false"},
			{Name: "output", Default: "json"},
			{Name: "output-dir", Default: "."},
			{Name: "project"},
			{Name: "provider"},
			{Name: "redact-sensitive-identifiers", Default: "false"},
			{Name: "resource-group"},
			{Name: "serve-report", Default: "auto"},
			{Name: "subscription-id"},
			{Name: "terminal-output", Default: "full"},
			{Name: "to-version", Required: true},
			{Name: "upgrade-context", Default: "unspecified"},
		}},
		{Path: "kubepreflight rollback"},
		{Path: "kubepreflight rollback assess", Flags: rollbackFlags()},
		{Path: "kubepreflight rollback plan", Flags: rollbackFlags()},
		{Path: "kubepreflight scan", Flags: []Flag{
			{Name: "allow-remote-report", Default: "false"},
			{Name: "cluster-name"},
			{Name: "collector-concurrency", Default: "4"},
			{Name: "collector-timeout", Default: "30s"},
			{Name: "context"},
			{Name: "findings-out", Default: "findings.json"},
			{Name: "helm-chart", Default: "[]"},
			{Name: "kubeconfig"},
			{Name: "listen", Default: "127.0.0.1:8080"},
			{Name: "location"},
			{Name: "manifests", Default: "[]"},
			{Name: "manifests-only", Default: "false"},
			{Name: "namespace-allowlist", Default: "[]"},
			{Name: "open-report", Default: "false"},
			{Name: "output", Default: "json"},
			{Name: "output-dir", Default: "."},
			{Name: "project"},
			{Name: "provider"},
			{Name: "redact-sensitive-identifiers", Default: "false"},
			{Name: "resource-group"},
			{Name: "serve-report", Default: "auto"},
			{Name: "subscription-id"},
			{Name: "target-version", Required: true},
			{Name: "terminal-output", Default: "full"},
			{Name: "upgrade-context", Default: "unspecified"},
		}},
		{Path: "kubepreflight version"},
	}
}

func rollbackFlags() []Flag {
	return []Flag{
		{Name: "assessment-out", Default: "rollback-assessment.json"},
		{Name: "cluster-name", Required: true},
		{Name: "collector-timeout", Default: "30s"},
		{Name: "findings"},
		{Name: "output", Default: "json"},
		{Name: "output-dir", Default: "."},
		{Name: "provider", Default: "eks", Required: true},
		{Name: "redact-sensitive-identifiers", Default: "false"},
		{Name: "terminal-output", Default: "full"},
	}
}

func ExpectedRuleIDs() []string {
	return []string{
		"API-001", "API-002", "WH-001", "WH-002", "WH-004", "WH-005",
		"DRAIN-001", "DRAIN-002", "DRAIN-003", "DRAIN-004", "DRAIN-005",
		"PDB-001", "PDB-002", "NODE-001", "NODE-002", "NODE-003", "NET-002",
		"WORKLOAD-001", "ADDON-001", "ADDON-002", "EKS-NG-001", "EKS-NG-002",
		"EKS-NG-003", "EKS-NG-004", "EKS-INSIGHT-001", "EKS-INSIGHT-002",
		"EKS-INSIGHT-003", "COREDNS-001", "CRD-001", "CRD-002", "APISERVICE-001",
	}
}

func ExpectedDefaultPriorities() map[string]string {
	return map[string]string{
		"API-001": "P2", "API-002": "P4", "WH-001": "P4", "WH-002": "P4",
		"WH-004": "P4", "WH-005": "P4", "DRAIN-001": "P3", "DRAIN-002": "P3",
		"DRAIN-003": "P3", "DRAIN-004": "P3", "DRAIN-005": "P3", "PDB-001": "P3",
		"PDB-002": "P3", "NODE-001": "P3", "NODE-002": "P2", "NODE-003": "P3",
		"NET-002": "P2", "WORKLOAD-001": "P4", "ADDON-001": "P2", "ADDON-002": "P3",
		"EKS-NG-001": "P4", "EKS-NG-002": "P3", "EKS-NG-003": "P4", "EKS-NG-004": "P4",
		"EKS-INSIGHT-001": "P2", "EKS-INSIGHT-002": "P4", "EKS-INSIGHT-003": "P4",
		"COREDNS-001": "P4", "CRD-001": "P2", "CRD-002": "P2", "APISERVICE-001": "P2",
	}
}
