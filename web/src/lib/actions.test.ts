import { expect, test } from "vitest";
import { buildActionGroups } from "./actions";
import type { Finding } from "./findings-schema";

function finding(ruleId: string, severity: Finding["severity"], name: string, globalBlocker = false): Finding {
  return { ruleId, severity, confidence: "OBSERVED", message: ruleId, evidence: [], remediation: `fix ${ruleId}`, fingerprint: ruleId, globalBlocker, resources: [{ plane: "live", kind: "ValidatingWebhookConfiguration", namespace: "", name }] };
}

test("groups related findings and orders global blockers first", () => {
  const groups = buildActionGroups([
    finding("PDB-001", "Blocker", "pdb"),
    finding("WH-001", "Warning", "guard"),
    finding("WH-002", "Blocker", "guard", true),
  ]);
  expect(groups).toHaveLength(2);
  expect(groups[0].ruleIds).toEqual(["WH-001", "WH-002"]);
  expect(groups[0].primary.ruleId).toBe("WH-002");
});

function manifestFinding(ruleId: string, kind: string, name: string, sourcePath: string): Finding {
  return {
    ruleId,
    severity: "Blocker",
    confidence: "STATIC_CERTAIN",
    message: ruleId,
    evidence: [],
    remediation: `fix ${ruleId}`,
    fingerprint: ruleId,
    resources: [{ plane: "manifest", kind, namespace: "", name, sourcePath, scope: "namespaced" }],
  };
}

// Guards the exact regression found in review: the manifest occurrence key
// previously collapsed to just sourcePath (dropping kind/name), so two
// unrelated resources declared without a namespace in the same manifest
// file incorrectly merged into one Next Action group.
test("two different resources in the same manifest file without a namespace do not merge", () => {
  const groups = buildActionGroups([
    manifestFinding("API-001", "PodDisruptionBudget", "old-pdb", "manifests/all.yaml"),
    manifestFinding("WH-001", "ValidatingWebhookConfiguration", "guard", "manifests/all.yaml"),
  ]);
  expect(groups).toHaveLength(2);
});

test("same resource (same manifest, kind, and name) with multiple findings still merges", () => {
  const groups = buildActionGroups([
    manifestFinding("API-001", "PodDisruptionBudget", "old-pdb", "manifests/all.yaml"),
    manifestFinding("PDB-001", "PodDisruptionBudget", "old-pdb", "manifests/all.yaml"),
  ]);
  expect(groups).toHaveLength(1);
  expect(groups[0].ruleIds).toEqual(["API-001", "PDB-001"]);
});

test("live resources with a UID still group correctly", () => {
  const groups = buildActionGroups([finding("WH-001", "Warning", "guard"), finding("WH-002", "Blocker", "guard", true)]);
  expect(groups).toHaveLength(1);
  expect(groups[0].ruleIds).toEqual(["WH-001", "WH-002"]);
});

test("manifest resources with the same name but different kind do not merge", () => {
  const groups = buildActionGroups([
    manifestFinding("API-001", "PodDisruptionBudget", "guard", "manifests/all.yaml"),
    manifestFinding("WH-001", "ValidatingWebhookConfiguration", "guard", "manifests/all.yaml"),
  ]);
  expect(groups).toHaveLength(2);
});

test("same kind/name in different manifest files does not incorrectly merge", () => {
  const groups = buildActionGroups([
    manifestFinding("API-001", "PodDisruptionBudget", "guard", "manifests/a.yaml"),
    manifestFinding("PDB-001", "PodDisruptionBudget", "guard", "manifests/b.yaml"),
  ]);
  expect(groups).toHaveLength(2);
});
