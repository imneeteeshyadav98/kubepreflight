package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
)

func TestRollbackRenderersExposeDecisionAndReasons(t *testing.T) {
	assessment := sampleRollbackAssessment()

	var terminal bytes.Buffer
	if err := WriteRollbackTerminal(&assessment, &terminal); err != nil {
		t.Fatalf("WriteRollbackTerminal: %v", err)
	}
	for _, want := range []string{"Recommendation: fix_forward_preferred", "MANAGED_NODEGROUP_ROLLBACK_REQUIRED"} {
		if !strings.Contains(terminal.String(), want) {
			t.Fatalf("terminal missing %q:\n%s", want, terminal.String())
		}
	}

	var markdown bytes.Buffer
	if err := WriteRollbackMarkdown(&assessment, &markdown); err != nil {
		t.Fatalf("WriteRollbackMarkdown: %v", err)
	}
	for _, want := range []string{"# KubePreflight Rollback Readiness", "`fix_forward_preferred`", "Reason Codes"} {
		if !strings.Contains(markdown.String(), want) {
			t.Fatalf("markdown missing %q:\n%s", want, markdown.String())
		}
	}

	var html bytes.Buffer
	if err := WriteRollbackHTML(&assessment, &html); err != nil {
		t.Fatalf("WriteRollbackHTML: %v", err)
	}
	for _, want := range []string{"KubePreflight Rollback Readiness", "fix forward preferred", "MANAGED_NODEGROUP_ROLLBACK_REQUIRED"} {
		if !strings.Contains(html.String(), want) {
			t.Fatalf("html missing %q:\n%s", want, html.String())
		}
	}
}

func TestWriteRollbackJSONRoundTripsSchema(t *testing.T) {
	assessment := sampleRollbackAssessment()
	var out bytes.Buffer
	if err := WriteRollbackJSON(&assessment, &out); err != nil {
		t.Fatalf("WriteRollbackJSON: %v", err)
	}

	var decoded rollback.Assessment
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.SchemaVersion != rollback.SchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", decoded.SchemaVersion, rollback.SchemaVersion)
	}
	if decoded.Recommendation.Decision != rollback.RecommendationFixForwardPreferred {
		t.Fatalf("decision = %q", decoded.Recommendation.Decision)
	}
}

// TestRollbackRenderersExposeAPIEvidenceTargetMismatch confirms the new
// ROLLBACK_EVIDENCE_TARGET_MISMATCH reason code and its Unknown check reach
// every rendering surface (terminal, markdown, HTML, JSON) through the same
// generic reason/check rendering every other rollback reason code already
// uses -- no renderer-specific special-casing is required for it.
func TestRollbackRenderersExposeAPIEvidenceTargetMismatch(t *testing.T) {
	assessment := sampleRollbackAssessment()
	assessment.Checks = append(assessment.Checks, rollback.Check{
		ID:          "reverse-compatibility",
		Title:       "API, CRD, and webhook state is compatible with rollback target",
		Status:      rollback.CheckUnknown,
		ReasonCodes: []rollback.ReasonCode{rollback.ReasonRollbackEvidenceTargetMismatch},
		Evidence:    []string{"API compatibility evidence target mismatch: findings target=1.36 rollback target=1.35"},
	})

	var terminal bytes.Buffer
	if err := WriteRollbackTerminal(&assessment, &terminal); err != nil {
		t.Fatalf("WriteRollbackTerminal: %v", err)
	}
	if !strings.Contains(terminal.String(), "ROLLBACK_EVIDENCE_TARGET_MISMATCH") {
		t.Fatalf("terminal missing mismatch reason code:\n%s", terminal.String())
	}

	var markdown bytes.Buffer
	if err := WriteRollbackMarkdown(&assessment, &markdown); err != nil {
		t.Fatalf("WriteRollbackMarkdown: %v", err)
	}
	if !strings.Contains(markdown.String(), "ROLLBACK_EVIDENCE_TARGET_MISMATCH") ||
		!strings.Contains(markdown.String(), "findings target=1.36 rollback target=1.35") {
		t.Fatalf("markdown missing mismatch reason code or evidence:\n%s", markdown.String())
	}

	var html bytes.Buffer
	if err := WriteRollbackHTML(&assessment, &html); err != nil {
		t.Fatalf("WriteRollbackHTML: %v", err)
	}
	if !strings.Contains(html.String(), "ROLLBACK_EVIDENCE_TARGET_MISMATCH") {
		t.Fatalf("html missing mismatch reason code:\n%s", html.String())
	}

	var out bytes.Buffer
	if err := WriteRollbackJSON(&assessment, &out); err != nil {
		t.Fatalf("WriteRollbackJSON: %v", err)
	}
	var decoded rollback.Assessment
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	found := false
	for _, check := range decoded.Checks {
		if check.ID != "reverse-compatibility" {
			continue
		}
		for _, reason := range check.ReasonCodes {
			if reason == rollback.ReasonRollbackEvidenceTargetMismatch {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("decoded JSON missing reverse-compatibility check with %s: %+v", rollback.ReasonRollbackEvidenceTargetMismatch, decoded.Checks)
	}
}

// TestRollbackRenderersExposeClusterEvidenceIdentityMismatch confirms the
// new ROLLBACK_EVIDENCE_CLUSTER_MISMATCH/ROLLBACK_EVIDENCE_CLUSTER_UNKNOWN
// reason codes and their Unknown checks reach every rendering surface
// (terminal, markdown, HTML, JSON) through the same generic reason/check
// rendering every other rollback reason code already uses -- mirrors
// TestRollbackRenderersExposeAPIEvidenceTargetMismatch's precedent from
// PR #207; no renderer-specific special-casing is required for it.
func TestRollbackRenderersExposeClusterEvidenceIdentityMismatch(t *testing.T) {
	assessment := sampleRollbackAssessment()
	assessment.Checks = append(assessment.Checks, rollback.Check{
		ID:          "disruption-readiness",
		Title:       "PDB and drain constraints do not block rollback preparation",
		Status:      rollback.CheckUnknown,
		ReasonCodes: []rollback.ReasonCode{rollback.ReasonRollbackEvidenceClusterMismatch},
		Evidence:    []string{"cluster identity mismatch: findings cluster=staging region=ap-south-1 vs assessed cluster=prod region=ap-south-1"},
	})

	var terminal bytes.Buffer
	if err := WriteRollbackTerminal(&assessment, &terminal); err != nil {
		t.Fatalf("WriteRollbackTerminal: %v", err)
	}
	if !strings.Contains(terminal.String(), "ROLLBACK_EVIDENCE_CLUSTER_MISMATCH") {
		t.Fatalf("terminal missing cluster mismatch reason code:\n%s", terminal.String())
	}

	var markdown bytes.Buffer
	if err := WriteRollbackMarkdown(&assessment, &markdown); err != nil {
		t.Fatalf("WriteRollbackMarkdown: %v", err)
	}
	if !strings.Contains(markdown.String(), "ROLLBACK_EVIDENCE_CLUSTER_MISMATCH") ||
		!strings.Contains(markdown.String(), "findings cluster=staging") {
		t.Fatalf("markdown missing cluster mismatch reason code or evidence:\n%s", markdown.String())
	}

	var html bytes.Buffer
	if err := WriteRollbackHTML(&assessment, &html); err != nil {
		t.Fatalf("WriteRollbackHTML: %v", err)
	}
	if !strings.Contains(html.String(), "ROLLBACK_EVIDENCE_CLUSTER_MISMATCH") {
		t.Fatalf("html missing cluster mismatch reason code:\n%s", html.String())
	}

	var out bytes.Buffer
	if err := WriteRollbackJSON(&assessment, &out); err != nil {
		t.Fatalf("WriteRollbackJSON: %v", err)
	}
	var decoded rollback.Assessment
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	found := false
	for _, check := range decoded.Checks {
		if check.ID != "disruption-readiness" {
			continue
		}
		for _, reason := range check.ReasonCodes {
			if reason == rollback.ReasonRollbackEvidenceClusterMismatch {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("decoded JSON missing disruption-readiness check with %s: %+v", rollback.ReasonRollbackEvidenceClusterMismatch, decoded.Checks)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded assessment with new reason code failed Validate(): %v", err)
	}
}

func sampleRollbackAssessment() rollback.Assessment {
	now := time.Date(2026, 7, 15, 8, 4, 0, 0, time.UTC)
	remaining := 360
	return rollback.Assessment{
		SchemaVersion: rollback.SchemaVersion,
		Mode:          rollback.ModePostUpgradeReadiness,
		Cluster: rollback.Cluster{
			Name:                  "prod",
			Region:                "ap-south-1",
			Provider:              "eks",
			CurrentVersion:        "1.36",
			RollbackTargetVersion: "1.35",
		},
		Eligibility: rollback.Eligibility{
			Status:           rollback.EligibilityEligible,
			RemainingMinutes: &remaining,
			Source:           "amazon-eks",
		},
		Readiness: rollback.Readiness{Status: rollback.ReadinessHighRisk, Warnings: 1},
		Recommendation: rollback.Recommendation{
			Decision:    rollback.RecommendationFixForwardPreferred,
			Confidence:  rollback.ConfidenceMedium,
			ReasonCodes: []rollback.ReasonCode{rollback.ReasonManagedNodegroupRollbackRequired},
		},
		Evidence: rollback.Evidence{Complete: true},
		Checks: []rollback.Check{{
			ID:          "managed-nodegroups",
			Title:       "Managed node groups are compatible with rollback target",
			Status:      rollback.CheckWarning,
			ReasonCodes: []rollback.ReasonCode{rollback.ReasonManagedNodegroupRollbackRequired},
			Evidence:    []string{"nodegroup apps version: 1.36"},
		}},
		GeneratedAt: now,
	}
}
