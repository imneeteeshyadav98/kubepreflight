package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/redact"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
)

// writeTempFindingsFile writes content to a fresh temp file and returns its
// path, for feeding to readFindingsReport / the rollback plan/assess
// commands without committing fixture artifacts to the repo.
func writeTempFindingsFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "findings.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

// realAssessmentJSON returns the JSON for a genuine rollback.Assessment
// document -- one of the confirmed cases that must NOT be accepted as a
// findings document, since it shares no structural fields with
// findings.Report other than the "schemaVersion" key name.
func realAssessmentJSON(t *testing.T) string {
	t.Helper()
	a := rollback.NewAssessment(rollback.ModePostUpgradeReadiness, time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC))
	a.Cluster = rollback.Cluster{Name: "prod", Provider: "eks", CurrentVersion: "1.35", RollbackTargetVersion: "1.34"}
	a.Eligibility = rollback.Eligibility{Status: rollback.EligibilityEligible, Source: "amazon-eks"}
	a.Readiness = rollback.Readiness{Status: rollback.ReadinessReady}
	a.Recommendation = rollback.Recommendation{Decision: rollback.RecommendationOperatorDecisionRequired, Confidence: rollback.ConfidenceMedium}
	a.Evidence = rollback.Evidence{Complete: true}
	return mustJSON(t, a)
}

// realComparisonJSON returns the JSON for a genuine comparison.Comparison
// document -- another confirmed wrong-document case. Its schemaVersion
// value happens to land in Report.SchemaVersion during a naive decode
// (same JSON key name), which is exactly the misleading case this PR
// closes.
func realComparisonJSON(t *testing.T) string {
	t.Helper()
	c := comparison.Comparison{
		SchemaVersion: comparison.SchemaVersion,
		Summary: comparison.Summary{
			BaselineVerdict: "pass",
			CurrentVerdict:  "pass",
		},
		New:       []comparison.Entry{},
		Resolved:  []comparison.Entry{},
		Changed:   []comparison.Changed{},
		Unchanged: []comparison.Entry{},
	}
	return mustJSON(t, c)
}

func canonicalLiveReport() *findings.Report {
	r := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), []findings.Finding{{
		RuleID:   "API-001",
		Severity: findings.SeverityBlocker,
		Message:  "removed API in target version",
	}})
	r.CurrentVersion = "1.33"
	r.EKSCluster = &findings.EKSClusterInfo{ClusterName: "prod", Region: "ap-south-1"}
	r.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageComplete},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageComplete},
	})
	return r
}

// TestReadFindingsReport_RejectsInvalidDocuments is the required 22-case
// invalid-input matrix: readFindingsReport must reject every one of these,
// and the error must never echo the file's raw contents.
func TestReadFindingsReport_RejectsInvalidDocuments(t *testing.T) {
	cleanReport := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)
	validJSON := mustJSON(t, cleanReport)

	cases := map[string]string{
		"empty file":                       "",
		"whitespace-only file":             "   \n\t\n  ",
		"invalid JSON":                     "{not json",
		"truncated JSON":                   `{"schemaVersion":"1.0","targetVersion":"1.34","findings":[`,
		"null":                             "null",
		"empty object":                     "{}",
		"arbitrary object":                 `{"hello":"world"}`,
		"array root":                       "[]",
		"primitive root":                   `"hello"`,
		"kubernetes object":                `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"example"}}`,
		"real rollback Assessment JSON":    realAssessmentJSON(t),
		"real comparison JSON":             realComparisonJSON(t),
		"missing schemaVersion":            `{"targetVersion":"1.34","findings":[]}`,
		"blank schemaVersion":              `{"schemaVersion":"   ","targetVersion":"1.34","findings":[]}`,
		"unsupported future schemaVersion": `{"schemaVersion":"9.9","targetVersion":"1.34","findings":[]}`,
		"supported schemaVersion, missing targetVersion":      `{"schemaVersion":"1.0","findings":[]}`,
		"supported schemaVersion, blank targetVersion":        `{"schemaVersion":"1.0","targetVersion":"   ","findings":[]}`,
		"supported schemaVersion, missing findings":           `{"schemaVersion":"1.0","targetVersion":"1.34"}`,
		"supported schemaVersion, findings null":              `{"schemaVersion":"1.0","targetVersion":"1.34","findings":null}`,
		"two concatenated JSON objects":                       validJSON + validJSON,
		"valid findings JSON followed by a second JSON value": validJSON + `{"hello":"world"}`,
		"valid findings JSON followed by trailing garbage":    validJSON + "not-json-garbage",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeTempFindingsFile(t, content)
			rpt, err := readFindingsReport(path)
			if err == nil {
				t.Fatalf("readFindingsReport(%s) succeeded, want an error; report = %+v", name, rpt)
			}
			if rpt != nil {
				t.Errorf("readFindingsReport(%s) returned a non-nil report alongside an error", name)
			}
			// The error must never echo the raw file contents back --
			// spot-check with a distinguishing marker unlikely to appear
			// in any legitimate error message.
			if strings.Contains(content, "not-json-garbage") && strings.Contains(err.Error(), "not-json-garbage") {
				t.Errorf("readFindingsReport(%s) error echoes file contents: %v", name, err)
			}
		})
	}
}

// TestReadFindingsReport_AcceptsValidDocuments is the required 9-case
// valid-input matrix: every genuine findings report variant must still be
// accepted, unknown fields must be ignored, and clean zero-finding reports
// must remain valid.
func TestReadFindingsReport_AcceptsValidDocuments(t *testing.T) {
	redacted := canonicalLiveReport()
	redact.Report(redacted)

	partial := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)
	partial.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"timed out listing PodDisruptionBudgets"}},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	})

	sameVersion := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)
	sameVersion.CurrentVersion = "1.34"

	cleanZeroFindings := findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)

	manifestOnly := findings.NewReport("1.34", "", "", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil)

	unknownTopLevel := mustJSON(t, cleanZeroFindings)
	// Splice an unknown additive top-level field into the object.
	unknownTopLevel = strings.Replace(unknownTopLevel, `"schemaVersion"`, `"futureTopLevelField":"some-value","schemaVersion"`, 1)

	cases := map[string]string{
		"canonical live findings report":                mustJSON(t, canonicalLiveReport()),
		"canonical manifest-only findings report":       mustJSON(t, manifestOnly),
		"redacted findings report":                      mustJSON(t, redacted),
		"partial-collection report":                     mustJSON(t, partial),
		"clean report with findings: []":                mustJSON(t, cleanZeroFindings),
		"same-version scan report":                      mustJSON(t, sameVersion),
		"unknown additive top-level fields":             unknownTopLevel,
		"unknown additive nested fields":                `{"schemaVersion":"1.0","targetVersion":"1.34","findings":[{"ruleId":"API-001","severity":"blocker","message":"x","resources":[],"futureNestedField":"some-value"}]}`,
		"valid report followed only by spaces/newlines": mustJSON(t, cleanZeroFindings) + "\n   \n",
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeTempFindingsFile(t, content)
			rpt, err := readFindingsReport(path)
			if err != nil {
				t.Fatalf("readFindingsReport(%s) = %v, want success", name, err)
			}
			if rpt == nil {
				t.Fatalf("readFindingsReport(%s) returned nil report without error", name)
			}
			if rpt.SchemaVersion != findings.SchemaVersion {
				t.Errorf("readFindingsReport(%s).SchemaVersion = %q, want %q", name, rpt.SchemaVersion, findings.SchemaVersion)
			}
			if strings.TrimSpace(rpt.TargetVersion) == "" {
				t.Errorf("readFindingsReport(%s).TargetVersion is blank", name)
			}
			if rpt.Findings == nil {
				t.Errorf("readFindingsReport(%s).Findings is nil, want non-nil (possibly empty) slice", name)
			}
		})
	}
}

// TestValidateRollbackFindingsDocument_UsesCanonicalSchemaConstant proves
// the reader's schema check is tied to findings.SchemaVersion -- the same
// constant NewReport/NewReportWithUpgradeContext stamp onto every genuine
// report -- rather than a second, independently-drifting "1.0" literal.
func TestValidateRollbackFindingsDocument_UsesCanonicalSchemaConstant(t *testing.T) {
	// A report built the same way every real scan builds one must pass
	// purely because it carries findings.SchemaVersion.
	genuine := findings.NewReport("1.34", "prod", "eks", time.Now().UTC(), nil)
	if genuine.SchemaVersion != findings.SchemaVersion {
		t.Fatalf("test precondition failed: NewReport did not stamp findings.SchemaVersion")
	}
	if err := validateRollbackFindingsDocument(*genuine); err != nil {
		t.Errorf("validateRollbackFindingsDocument(genuine NewReport output) = %v, want nil", err)
	}

	// A document whose schemaVersion is anything other than
	// findings.SchemaVersion -- including a value that would only pass a
	// hardcoded "1.0" check by coincidence -- must be rejected, and the
	// rejection message must name the actual canonical constant value.
	wrong := *genuine
	wrong.SchemaVersion = findings.SchemaVersion + "-bogus"
	err := validateRollbackFindingsDocument(wrong)
	if err == nil {
		t.Fatal("validateRollbackFindingsDocument(wrong schemaVersion) succeeded, want an error")
	}
	if !strings.Contains(err.Error(), findings.SchemaVersion) {
		t.Errorf("error = %v, want it to reference the canonical constant %q", err, findings.SchemaVersion)
	}
}

// TestReadFindingsReport_TrailingContentRejected focuses specifically on
// the trailing-document boundary check called out in the PR: exactly one
// JSON document must be accepted, via a second decode requiring io.EOF.
func TestReadFindingsReport_TrailingContentRejected(t *testing.T) {
	valid := mustJSON(t, findings.NewReport("1.34", "prod", "eks", time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC), nil))

	t.Run("two concatenated objects", func(t *testing.T) {
		path := writeTempFindingsFile(t, valid+valid)
		if _, err := readFindingsReport(path); err == nil {
			t.Fatal("readFindingsReport succeeded on two concatenated JSON objects, want an error")
		}
	})

	t.Run("followed by a second JSON value", func(t *testing.T) {
		path := writeTempFindingsFile(t, valid+`{"another":"document"}`)
		if _, err := readFindingsReport(path); err == nil {
			t.Fatal("readFindingsReport succeeded with a second trailing JSON value, want an error")
		}
	})

	t.Run("followed by trailing garbage", func(t *testing.T) {
		path := writeTempFindingsFile(t, valid+"garbage-not-json")
		if _, err := readFindingsReport(path); err == nil {
			t.Fatal("readFindingsReport succeeded with trailing non-JSON garbage, want an error")
		}
	})

	t.Run("followed only by whitespace remains accepted", func(t *testing.T) {
		path := writeTempFindingsFile(t, valid+"\n\n   \t\n")
		if _, err := readFindingsReport(path); err != nil {
			t.Fatalf("readFindingsReport failed on trailing whitespace-only content: %v", err)
		}
	})
}

// TestRollbackPlanCommand_InvalidFindingsExitsFourBeforeCollection and its
// assess counterpart below guard the command-level contract: a malformed
// --findings must fail with exit code 4 (infrastructure failure), and it
// must fail BEFORE the EKS collector is ever invoked -- proven here by the
// absence of any "AWS credentials"/collector-related error text, since
// this test environment has no AWS credentials configured and a collector
// attempt would otherwise surface that unrelated error first.
func TestRollbackPlanCommand_InvalidFindingsExitsFourBeforeCollection(t *testing.T) {
	badFindings := writeTempFindingsFile(t, `{"hello":"world"}`)
	dir := t.TempDir()

	cmd := newRollbackCmd(new(int))
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{
		"plan",
		"--cluster-name", "prod",
		"--findings", badFindings,
		"--output-dir", dir,
		"--terminal-output", "silent",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("rollback plan --findings <wrong document> succeeded, want an error")
	}
	if !isInfraFailure(err) {
		t.Errorf("rollback plan --findings error = %v, not marked as an infrastructure failure", err)
	}
	if got := exitCodeForError(err, 0); got != 4 {
		t.Errorf("exitCodeForError(rollback plan findings error) = %d, want 4", got)
	}
	if strings.Contains(err.Error(), "AWS credentials") || strings.Contains(err.Error(), "EKS rollback collector") {
		t.Errorf("error = %v, want findings validation to fail before EKS collection is attempted", err)
	}
	if !strings.Contains(err.Error(), "invalid --findings document") {
		t.Errorf("error = %v, want it to identify an invalid --findings document", err)
	}
}

func TestRollbackAssessCommand_InvalidFindingsExitsFourBeforeCollection(t *testing.T) {
	badFindings := writeTempFindingsFile(t, "not json at all")
	dir := t.TempDir()

	cmd := newRollbackCmd(new(int))
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{
		"assess",
		"--cluster-name", "prod",
		"--findings", badFindings,
		"--output-dir", dir,
		"--terminal-output", "silent",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("rollback assess --findings <malformed JSON> succeeded, want an error")
	}
	if !isInfraFailure(err) {
		t.Errorf("rollback assess --findings error = %v, not marked as an infrastructure failure", err)
	}
	if got := exitCodeForError(err, 0); got != 4 {
		t.Errorf("exitCodeForError(rollback assess findings error) = %d, want 4", got)
	}
	if strings.Contains(err.Error(), "AWS credentials") || strings.Contains(err.Error(), "EKS rollback collector") {
		t.Errorf("error = %v, want findings validation to fail before EKS collection is attempted", err)
	}
}

// TestRollbackCommand_NoOutputFilesWrittenOnInvalidFindings guards that no
// assessment/report output is generated when --findings input validation
// fails.
func TestRollbackCommand_NoOutputFilesWrittenOnInvalidFindings(t *testing.T) {
	badFindings := writeTempFindingsFile(t, "[]")
	dir := t.TempDir()

	cmd := newRollbackCmd(new(int))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"plan",
		"--cluster-name", "prod",
		"--findings", badFindings,
		"--output-dir", dir,
		"--terminal-output", "silent",
	})
	if err := cmd.Execute(); err == nil {
		t.Fatal("rollback plan --findings [] succeeded, want an error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("output directory has %d entries after a findings validation failure, want 0: %v", len(entries), entries)
	}
}
