package comparison

import (
	"encoding/json"
	"fmt"

	"kubepreflight/internal/findings"
)

// LoadAndNormalize parses raw findings.json bytes and backfills fields an
// older schema version might not have written, so a baseline captured
// months ago can still be compared against a current scan:
//
//   - missing Priority/PriorityReason/AffectedScope/CanUpgradeContinue on a
//     finding are derived via findings.AssignPriority, the same function
//     NewReport itself uses -- never guessed, never left zero-valued.
//   - a missing UpgradeReadiness/APICompatibility summary is rebuilt from
//     the document's own findings and verdict, exactly as NewReport builds
//     it, so an old document without these fields still gets an accurate
//     one instead of a nil comparison target.
//   - ScanCoverage is preserved exactly as recorded -- an old document's
//     INCOMPLETE status is never silently treated as CLEAN just because a
//     newer field happened to be absent too.
//   - unknown/future fields are tolerated (encoding/json already ignores
//     them; this package never rejects a document just for having fields
//     it doesn't know about).
//
// A document that isn't recognizable as a findings.json at all --
// malformed JSON, or missing the fields every schema version has always
// had (schemaVersion, targetVersion) -- is an explicit error, never a
// silently-empty comparison.
func LoadAndNormalize(raw []byte) (*findings.Report, error) {
	var r findings.Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parsing findings document: %w", err)
	}
	if r.SchemaVersion == "" || r.TargetVersion == "" {
		return nil, fmt.Errorf("not a recognizable findings.json document: missing schemaVersion/targetVersion")
	}

	for i, f := range r.Findings {
		if f.Priority == "" {
			r.Findings[i] = findings.AssignPriority(f)
		}
	}

	verdict := r.Result()
	if r.APICompatibility == nil {
		r.APICompatibility = findings.BuildAPICompatibilitySummary(r.Findings)
	}
	if r.UpgradeReadiness == nil {
		r.UpgradeReadiness = findings.BuildUpgradeReadinessSummary(r.Findings, verdict)
	}

	return &r, nil
}
