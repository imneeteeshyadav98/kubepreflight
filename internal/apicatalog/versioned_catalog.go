package apicatalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed versioned_catalog.json
var defaultVersionedCatalogJSON []byte

const VersionedSchemaVersion = "apicatalog.kubepreflight.io/v1"

type VersionedDocument struct {
	SchemaVersion string         `json:"schemaVersion"`
	Entries       []VersionedAPI `json:"entries"`

	// BuildSupportedTargetRange is the Kubernetes target-version range this
	// catalog build's maintainer has explicitly verified the embedded data
	// against. It is declared independently of any single entry's own
	// SupportedTargetRange — individual entries may cover a narrower or
	// wider window than this — so it stays stable and unambiguous as
	// entries are added, removed, or re-ranged. Callers that need a
	// fail-fast "is this target version one we can speak to at all" gate
	// (as opposed to a per-API removed/deprecated decision) should use
	// this, via VersionedCatalog.TargetSupported, rather than trying to
	// infer a range from Entries().
	BuildSupportedTargetRange SupportedTargetRange `json:"buildSupportedTargetRange"`
}

type VersionedAPI struct {
	Group                 string               `json:"group"`
	Version               string               `json:"version"`
	Resource              string               `json:"resource"`
	Kind                  string               `json:"kind"`
	Namespaced            bool                 `json:"namespaced"`
	DeprecatedInVersion   string               `json:"deprecatedInVersion"`
	RemovedInVersion      string               `json:"removedInVersion"`
	ReplacementAPI        string               `json:"replacementAPI"`
	ReplacementAPIVersion string               `json:"replacementAPIVersion,omitempty"`
	SupportedTargetRange  SupportedTargetRange `json:"supportedTargetRange"`
	Source                string               `json:"source"`
	Reference             string               `json:"reference"`
	LastVerifiedDate      string               `json:"lastVerifiedDate"`
	Confidence            string               `json:"confidence"`
}

type SupportedTargetRange struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

type VersionedCatalog struct {
	entries    []VersionedAPI
	buildRange SupportedTargetRange
}

type versionedKey struct {
	group   string
	version string
	kind    string
}

var (
	versionedKubernetesVersionRE = regexp.MustCompile(`^1\.[0-9]+$`)
	versionedAPIVersionRE        = regexp.MustCompile(`^v[0-9]+((alpha|beta)[0-9]+)?$`)
)

var defaultVersionedCatalog, defaultVersionedCatalogErr = func() (*VersionedCatalog, error) {
	return LoadVersionedJSON(defaultVersionedCatalogJSON)
}()

func DefaultVersioned() (*VersionedCatalog, error) {
	if defaultVersionedCatalogErr != nil {
		return nil, defaultVersionedCatalogErr
	}
	if defaultVersionedCatalog == nil {
		return nil, fmt.Errorf("default API version catalog was not initialized")
	}
	return defaultVersionedCatalog, nil
}

func LoadVersionedJSON(raw []byte) (*VersionedCatalog, error) {
	var doc VersionedDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode API version catalog: %w", err)
	}
	return NewVersioned(doc)
}

func NewVersioned(doc VersionedDocument) (*VersionedCatalog, error) {
	if doc.SchemaVersion != VersionedSchemaVersion {
		return nil, fmt.Errorf("unsupported API version catalog schemaVersion %q", doc.SchemaVersion)
	}
	entries, err := normalizeAndValidateVersioned(doc.Entries)
	if err != nil {
		return nil, err
	}
	buildRange, err := normalizeBuildSupportedTargetRange(doc.BuildSupportedTargetRange)
	if err != nil {
		return nil, err
	}
	return &VersionedCatalog{entries: entries, buildRange: buildRange}, nil
}

func normalizeBuildSupportedTargetRange(r SupportedTargetRange) (SupportedTargetRange, error) {
	r.Min = strings.TrimPrefix(strings.TrimSpace(r.Min), "v")
	r.Max = strings.TrimPrefix(strings.TrimSpace(r.Max), "v")
	if !validKubernetesVersion(r.Min) || !validKubernetesVersion(r.Max) {
		return SupportedTargetRange{}, fmt.Errorf("buildSupportedTargetRange must use Kubernetes major.minor versions")
	}
	if compareKubernetesVersions(r.Min, r.Max) > 0 {
		return SupportedTargetRange{}, fmt.Errorf("buildSupportedTargetRange min %s is after max %s", r.Min, r.Max)
	}
	return r, nil
}

// TargetSupported reports whether targetVersion falls within this catalog
// build's declared BuildSupportedTargetRange — the range explicitly
// verified for this build, independent of any single entry's own
// SupportedTargetRange. min/max are the normalized declared bounds
// (returned even when ok is false, for use in a "supported range is X-Y"
// error message). A malformed targetVersion reports ok=false.
func (c *VersionedCatalog) TargetSupported(targetVersion string) (min, max string, ok bool) {
	if c == nil {
		return "", "", false
	}
	target, valid := normalizeVersionedKubernetesVersion(targetVersion)
	if !valid {
		return c.buildRange.Min, c.buildRange.Max, false
	}
	return c.buildRange.Min, c.buildRange.Max, versionInRange(target, c.buildRange.Min, c.buildRange.Max)
}

func (c *VersionedCatalog) Entries() []VersionedAPI {
	if c == nil || len(c.entries) == 0 {
		return nil
	}
	out := make([]VersionedAPI, len(c.entries))
	copy(out, c.entries)
	return out
}

func (c *VersionedCatalog) Lookup(group, version, kind, targetVersion string) (VersionedAPI, bool) {
	if c == nil {
		return VersionedAPI{}, false
	}
	group = normalizeVersionedGroup(group)
	version = normalizeVersionedAPIVersion(version)
	kind = normalizeVersionedKind(kind)
	target, ok := normalizeVersionedKubernetesVersion(targetVersion)
	if !ok {
		return VersionedAPI{}, false
	}
	for _, entry := range c.entries {
		if entry.Group != group || entry.Version != version || entry.Kind != kind {
			continue
		}
		if versionInRange(target, entry.SupportedTargetRange.Min, entry.SupportedTargetRange.Max) {
			return entry, true
		}
	}
	return VersionedAPI{}, false
}

func normalizeAndValidateVersioned(entries []VersionedAPI) ([]VersionedAPI, error) {
	out := make([]VersionedAPI, 0, len(entries))
	for i, entry := range entries {
		normalized, err := normalizeVersionedEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		out = append(out, normalized)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		if out[i].Version != out[j].Version {
			return out[i].Version < out[j].Version
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].SupportedTargetRange.Min != out[j].SupportedTargetRange.Min {
			return compareKubernetesVersions(out[i].SupportedTargetRange.Min, out[j].SupportedTargetRange.Min) < 0
		}
		return compareKubernetesVersions(out[i].SupportedTargetRange.Max, out[j].SupportedTargetRange.Max) < 0
	})

	for i := 1; i < len(out); i++ {
		prev, cur := out[i-1], out[i]
		if versionedEntryKey(prev) != versionedEntryKey(cur) {
			continue
		}
		if rangesOverlap(prev.SupportedTargetRange, cur.SupportedTargetRange) {
			return nil, fmt.Errorf("%s/%s %s has overlapping supported target ranges %s-%s and %s-%s",
				cur.Group, cur.Version, cur.Kind,
				prev.SupportedTargetRange.Min, prev.SupportedTargetRange.Max,
				cur.SupportedTargetRange.Min, cur.SupportedTargetRange.Max)
		}
	}
	return out, nil
}

func normalizeVersionedEntry(entry VersionedAPI) (VersionedAPI, error) {
	entry.Group = normalizeVersionedGroup(entry.Group)
	entry.Version = normalizeVersionedAPIVersion(entry.Version)
	entry.Resource = strings.ToLower(strings.TrimSpace(entry.Resource))
	entry.Kind = normalizeVersionedKind(entry.Kind)
	entry.DeprecatedInVersion = strings.TrimPrefix(strings.TrimSpace(entry.DeprecatedInVersion), "v")
	entry.RemovedInVersion = strings.TrimPrefix(strings.TrimSpace(entry.RemovedInVersion), "v")
	entry.ReplacementAPI = strings.TrimSpace(entry.ReplacementAPI)
	entry.ReplacementAPIVersion = strings.TrimSpace(entry.ReplacementAPIVersion)
	entry.SupportedTargetRange.Min = strings.TrimPrefix(strings.TrimSpace(entry.SupportedTargetRange.Min), "v")
	entry.SupportedTargetRange.Max = strings.TrimPrefix(strings.TrimSpace(entry.SupportedTargetRange.Max), "v")
	entry.Source = strings.TrimSpace(entry.Source)
	entry.Reference = strings.TrimSpace(entry.Reference)
	entry.LastVerifiedDate = strings.TrimSpace(entry.LastVerifiedDate)
	entry.Confidence = strings.TrimSpace(entry.Confidence)

	if entry.Version == "" || !versionedAPIVersionRE.MatchString(entry.Version) {
		return VersionedAPI{}, fmt.Errorf("invalid API version %q", entry.Version)
	}
	if entry.Resource == "" {
		return VersionedAPI{}, fmt.Errorf("resource is required")
	}
	if entry.Kind == "" {
		return VersionedAPI{}, fmt.Errorf("kind is required")
	}
	if !validKubernetesVersion(entry.DeprecatedInVersion) {
		return VersionedAPI{}, fmt.Errorf("invalid deprecatedInVersion %q", entry.DeprecatedInVersion)
	}
	if !validKubernetesVersion(entry.RemovedInVersion) {
		return VersionedAPI{}, fmt.Errorf("invalid removedInVersion %q", entry.RemovedInVersion)
	}
	if compareKubernetesVersions(entry.DeprecatedInVersion, entry.RemovedInVersion) > 0 {
		return VersionedAPI{}, fmt.Errorf("deprecatedInVersion %s is after removedInVersion %s", entry.DeprecatedInVersion, entry.RemovedInVersion)
	}
	if entry.ReplacementAPI == "" {
		return VersionedAPI{}, fmt.Errorf("replacementAPI is required")
	}
	if !validKubernetesVersion(entry.SupportedTargetRange.Min) || !validKubernetesVersion(entry.SupportedTargetRange.Max) {
		return VersionedAPI{}, fmt.Errorf("supportedTargetRange must use Kubernetes major.minor versions")
	}
	if compareKubernetesVersions(entry.SupportedTargetRange.Min, entry.SupportedTargetRange.Max) > 0 {
		return VersionedAPI{}, fmt.Errorf("supportedTargetRange min %s is after max %s", entry.SupportedTargetRange.Min, entry.SupportedTargetRange.Max)
	}
	if entry.Source == "" {
		return VersionedAPI{}, fmt.Errorf("source is required")
	}
	if entry.Reference == "" {
		return VersionedAPI{}, fmt.Errorf("reference is required")
	}
	if _, err := time.Parse("2006-01-02", entry.LastVerifiedDate); err != nil {
		return VersionedAPI{}, fmt.Errorf("invalid lastVerifiedDate %q", entry.LastVerifiedDate)
	}
	if entry.Confidence != "STATIC_CERTAIN" {
		return VersionedAPI{}, fmt.Errorf("unsupported confidence %q", entry.Confidence)
	}
	return entry, nil
}

func versionedEntryKey(entry VersionedAPI) versionedKey {
	return versionedKey{group: entry.Group, version: entry.Version, kind: entry.Kind}
}

func normalizeVersionedGroup(group string) string {
	return strings.ToLower(strings.TrimSpace(group))
}

func normalizeVersionedAPIVersion(version string) string {
	return strings.ToLower(strings.TrimSpace(version))
}

func normalizeVersionedKind(kind string) string {
	return strings.TrimSpace(kind)
}

func normalizeVersionedKubernetesVersion(v string) (string, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return "", false
	}
	normalized := parts[0] + "." + parts[1]
	if !validKubernetesVersion(normalized) {
		return "", false
	}
	return normalized, true
}

func validKubernetesVersion(v string) bool {
	return versionedKubernetesVersionRE.MatchString(v)
}

func versionInRange(v, min, max string) bool {
	return compareKubernetesVersions(v, min) >= 0 && compareKubernetesVersions(v, max) <= 0
}

func rangesOverlap(a, b SupportedTargetRange) bool {
	return compareKubernetesVersions(a.Min, b.Max) <= 0 && compareKubernetesVersions(b.Min, a.Max) <= 0
}

func compareKubernetesVersions(a, b string) int {
	aMajor, aMinor := mustParseKubernetesVersion(a)
	bMajor, bMinor := mustParseKubernetesVersion(b)
	if aMajor != bMajor {
		if aMajor < bMajor {
			return -1
		}
		return 1
	}
	if aMinor < bMinor {
		return -1
	}
	if aMinor > bMinor {
		return 1
	}
	return 0
}

func mustParseKubernetesVersion(v string) (major, minor int) {
	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid Kubernetes version %q", v))
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		panic(fmt.Sprintf("invalid Kubernetes version %q", v))
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		panic(fmt.Sprintf("invalid Kubernetes version %q", v))
	}
	return major, minor
}
