package compatcatalog

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

//go:embed catalog.json
var defaultCatalogJSON []byte

const SchemaVersion = "compatcatalog.kubepreflight.io/v1"

type Document struct {
	SchemaVersion string  `json:"schemaVersion"`
	Entries       []Entry `json:"entries"`
}

type Entry struct {
	KubernetesVersion        string `json:"kubernetesVersion"`
	Provider                 string `json:"provider"`
	AddonName                string `json:"addonName"`
	MinimumCompatibleVersion string `json:"minimumCompatibleVersion"`
	RecommendedVersion       string `json:"recommendedVersion"`
	Source                   string `json:"source"`
	Reference                string `json:"reference"`
	LastVerifiedDate         string `json:"lastVerifiedDate"`
	Confidence               string `json:"confidence"`
}

type Catalog struct {
	entries []Entry
	byKey   map[key]Entry
}

type key struct {
	provider          string
	addonName         string
	kubernetesVersion string
}

var (
	kubernetesVersionRE = regexp.MustCompile(`^1\.[0-9]+$`)
	versionTokenRE      = regexp.MustCompile(`[0-9]+|[A-Za-z]+`)
)

var defaultCatalog, defaultCatalogErr = func() (*Catalog, error) {
	return LoadJSON(defaultCatalogJSON)
}()

func Default() (*Catalog, error) {
	if defaultCatalogErr != nil {
		return nil, defaultCatalogErr
	}
	if defaultCatalog == nil {
		return nil, fmt.Errorf("default compatibility catalog was not initialized")
	}
	return defaultCatalog, nil
}

func LoadJSON(raw []byte) (*Catalog, error) {
	var doc Document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode compatibility catalog: %w", err)
	}
	return New(doc)
}

func New(doc Document) (*Catalog, error) {
	if doc.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported compatibility catalog schemaVersion %q", doc.SchemaVersion)
	}
	entries, err := normalizeAndValidate(doc.Entries)
	if err != nil {
		return nil, err
	}
	c := &Catalog{entries: entries, byKey: make(map[key]Entry, len(entries))}
	for _, entry := range entries {
		c.byKey[entryKey(entry.Provider, entry.AddonName, entry.KubernetesVersion)] = entry
	}
	return c, nil
}

func (c *Catalog) Entries() []Entry {
	if c == nil || len(c.entries) == 0 {
		return nil
	}
	out := make([]Entry, len(c.entries))
	copy(out, c.entries)
	return out
}

func (c *Catalog) Lookup(provider, addonName, kubernetesVersion string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	entry, ok := c.byKey[entryKey(provider, addonName, kubernetesVersion)]
	return entry, ok
}

func (e Entry) InstalledStatus(installedVersion string) Status {
	if strings.TrimSpace(installedVersion) == "" || !looksLikeVersion(installedVersion) {
		return StatusUnknown
	}
	if CompareVersions(installedVersion, e.MinimumCompatibleVersion) < 0 {
		return StatusIncompatible
	}
	if CompareVersions(installedVersion, e.RecommendedVersion) < 0 {
		return StatusUpgradeRecommended
	}
	return StatusCompatible
}

type Status string

const (
	StatusCompatible         Status = "compatible"
	StatusIncompatible       Status = "incompatible"
	StatusUpgradeRecommended Status = "upgrade-recommended"
	StatusUnknown            Status = "unknown"
)

func normalizeAndValidate(entries []Entry) ([]Entry, error) {
	normalized := make([]Entry, len(entries))
	seen := map[key]bool{}
	for i, entry := range entries {
		n, err := normalizeEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("entries[%d]: %w", i, err)
		}
		k := entryKey(n.Provider, n.AddonName, n.KubernetesVersion)
		if seen[k] {
			return nil, fmt.Errorf("entries[%d]: duplicate catalog entry for provider=%q addonName=%q kubernetesVersion=%q", i, n.Provider, n.AddonName, n.KubernetesVersion)
		}
		seen[k] = true
		normalized[i] = n
	}
	sort.Slice(normalized, func(i, j int) bool {
		ki, kj := entryKey(normalized[i].Provider, normalized[i].AddonName, normalized[i].KubernetesVersion), entryKey(normalized[j].Provider, normalized[j].AddonName, normalized[j].KubernetesVersion)
		if ki.provider != kj.provider {
			return ki.provider < kj.provider
		}
		if ki.addonName != kj.addonName {
			return ki.addonName < kj.addonName
		}
		return ki.kubernetesVersion < kj.kubernetesVersion
	})
	return normalized, nil
}

func normalizeEntry(entry Entry) (Entry, error) {
	entry.Provider = strings.ToLower(strings.TrimSpace(entry.Provider))
	entry.AddonName = strings.ToLower(strings.TrimSpace(entry.AddonName))
	entry.KubernetesVersion = normalizeKubernetesVersion(entry.KubernetesVersion)
	entry.MinimumCompatibleVersion = strings.TrimSpace(entry.MinimumCompatibleVersion)
	entry.RecommendedVersion = strings.TrimSpace(entry.RecommendedVersion)
	entry.Source = strings.TrimSpace(entry.Source)
	entry.Reference = strings.TrimSpace(entry.Reference)
	entry.LastVerifiedDate = strings.TrimSpace(entry.LastVerifiedDate)
	entry.Confidence = strings.TrimSpace(entry.Confidence)

	if !kubernetesVersionRE.MatchString(entry.KubernetesVersion) {
		return entry, fmt.Errorf("kubernetesVersion %q must be major.minor, e.g. 1.34", entry.KubernetesVersion)
	}
	if entry.Provider == "" {
		return entry, fmt.Errorf("provider is required")
	}
	if entry.AddonName == "" {
		return entry, fmt.Errorf("addonName is required")
	}
	if !looksLikeVersion(entry.MinimumCompatibleVersion) {
		return entry, fmt.Errorf("minimumCompatibleVersion %q is not a parseable version", entry.MinimumCompatibleVersion)
	}
	if !looksLikeVersion(entry.RecommendedVersion) {
		return entry, fmt.Errorf("recommendedVersion %q is not a parseable version", entry.RecommendedVersion)
	}
	if CompareVersions(entry.MinimumCompatibleVersion, entry.RecommendedVersion) > 0 {
		return entry, fmt.Errorf("minimumCompatibleVersion %q must not be greater than recommendedVersion %q", entry.MinimumCompatibleVersion, entry.RecommendedVersion)
	}
	if entry.Source == "" {
		return entry, fmt.Errorf("source is required")
	}
	if entry.Reference == "" {
		return entry, fmt.Errorf("reference is required")
	}
	if _, err := time.Parse("2006-01-02", entry.LastVerifiedDate); err != nil {
		return entry, fmt.Errorf("lastVerifiedDate %q must be YYYY-MM-DD", entry.LastVerifiedDate)
	}
	switch entry.Confidence {
	case "STATIC_CERTAIN", "PROVIDER_REPORTED", "OBSERVED":
	default:
		return entry, fmt.Errorf("confidence %q is not supported", entry.Confidence)
	}
	return entry, nil
}

func entryKey(provider, addonName, kubernetesVersion string) key {
	return key{
		provider:          strings.ToLower(strings.TrimSpace(provider)),
		addonName:         strings.ToLower(strings.TrimSpace(addonName)),
		kubernetesVersion: normalizeKubernetesVersion(kubernetesVersion),
	}
}

func normalizeKubernetesVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

func looksLikeVersion(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" || strings.EqualFold(version, "latest") || strings.Contains(version, "@sha256:") {
		return false
	}
	return len(versionTokenRE.FindAllString(version, -1)) > 0 && strings.ContainsAny(version, "0123456789")
}

func CompareVersions(a, b string) int {
	at, bt := versionTokens(a), versionTokens(b)
	max := len(at)
	if len(bt) > max {
		max = len(bt)
	}
	for i := 0; i < max; i++ {
		var av, bv token
		if i < len(at) {
			av = at[i]
		}
		if i < len(bt) {
			bv = bt[i]
		}
		if av.kind != bv.kind {
			if av.kind == tokenNumber {
				return 1
			}
			if bv.kind == tokenNumber {
				return -1
			}
		}
		if av.value != bv.value {
			if av.value < bv.value {
				return -1
			}
			return 1
		}
	}
	return 0
}

type tokenKind int

const (
	tokenEmpty tokenKind = iota
	tokenString
	tokenNumber
)

type token struct {
	kind  tokenKind
	value string
}

func versionTokens(version string) []token {
	matches := versionTokenRE.FindAllString(strings.ToLower(strings.TrimSpace(version)), -1)
	out := make([]token, 0, len(matches))
	for _, match := range matches {
		if n, err := strconv.Atoi(match); err == nil {
			out = append(out, token{kind: tokenNumber, value: fmt.Sprintf("%020d", n)})
			continue
		}
		out = append(out, token{kind: tokenString, value: match})
	}
	return out
}
