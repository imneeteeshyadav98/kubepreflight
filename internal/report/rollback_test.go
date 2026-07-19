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
