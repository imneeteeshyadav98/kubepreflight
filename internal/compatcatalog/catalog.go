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

// RequiredAddon is one add-on the compatibility catalog is expected to
// cover for every Kubernetes target version it models, and the provider
// scope its entries must use. This is the single source of truth for two
// separate things that must never drift apart: knownAddonProviders (which
// entryKey/normalizeEntry uses to reject a known add-on filed under the
// wrong provider -- a data-entry mistake, e.g. cert-manager accidentally
// entered as provider "eks") and MissingRequiredEntries (which reports a
// known add-on that's simply absent for some target version the catalog
// otherwise covers).
type RequiredAddon struct {
	Provider  string
	AddonName string
}

// RequiredAddons lists every add-on this catalog is expected to cover for
// every Kubernetes target version it models. Provider-specific add-ons
// (currently only aws-load-balancer-controller, which only exists on EKS)
// are required under their own provider only -- MissingRequiredEntries
// never expects an "eks"-scoped add-on to also have a "kubernetes"-scoped
// entry, or vice versa.
var RequiredAddons = []RequiredAddon{
	{Provider: "eks", AddonName: "vpc-cni"},
	{Provider: "eks", AddonName: "kube-proxy"},
	{Provider: "eks", AddonName: "coredns"},
	{Provider: "eks", AddonName: "aws-ebs-csi-driver"},
	{Provider: "eks", AddonName: "aws-efs-csi-driver"},
	{Provider: "eks", AddonName: "aws-load-balancer-controller"},
	{Provider: "kubernetes", AddonName: "metrics-server"},
	{Provider: "kubernetes", AddonName: "ingress-nginx"},
	{Provider: "kubernetes", AddonName: "cert-manager"},
	{Provider: "kubernetes", AddonName: "external-dns"},
}

// knownAddonProviders maps each RequiredAddons entry's add-on name to its
// single canonical provider, derived once at package init. normalizeEntry
// rejects any catalog entry for a known add-on name filed under a
// different provider -- catching a data-entry mistake (e.g. cert-manager,
// which is ordinary cluster-agnostic software, accidentally entered under
// provider "eks") at catalog-load time rather than silently producing a
// catalog entry that Lookup can never actually be reached by the rules
// that query it under the correct provider.
var knownAddonProviders = func() map[string]string {
	out := make(map[string]string, len(RequiredAddons))
	for _, addon := range RequiredAddons {
		out[addon.AddonName] = addon.Provider
	}
	return out
}()

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

// TargetVersions returns every distinct Kubernetes target version present
// in the catalog, sorted ascending by major.minor. Used by
// MissingRequiredEntries to decide which versions "every catalog-supported
// target version" actually refers to -- entirely data-driven, so adding a
// new target version's entries automatically extends what coverage is
// checked without any code change.
func (c *Catalog) TargetVersions() []string {
	if c == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, entry := range c.entries {
		if !seen[entry.KubernetesVersion] {
			seen[entry.KubernetesVersion] = true
			out = append(out, entry.KubernetesVersion)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		iMajor, iMinor, _ := parseTargetVersionParts(out[i])
		jMajor, jMinor, _ := parseTargetVersionParts(out[j])
		if iMajor != jMajor {
			return iMajor < jMajor
		}
		return iMinor < jMinor
	})
	return out
}

func parseTargetVersionParts(v string) (major, minor int, err error) {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("cannot parse %q", v)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minor, err = strconv.Atoi(parts[1])
	return major, minor, err
}

// MissingRequiredEntries reports, for every target version TargetVersions
// returns, every RequiredAddons entry the catalog has no corresponding
// entry for -- one line per gap, sorted deterministically (target version,
// then provider, then add-on name). An empty result means every
// catalog-supported target version has full required coverage. This is a
// coverage report, not a validation error on its own: a brand-new target
// version legitimately starts with zero entries and gets filled in over
// time (see docs/compatibility-catalog.md's maintenance process) -- callers
// that want this to be a hard gate (e.g. cmd/compatcatalogcheck) decide
// that themselves.
func (c *Catalog) MissingRequiredEntries() []string {
	if c == nil {
		return nil
	}
	var missing []string
	for _, target := range c.TargetVersions() {
		for _, addon := range RequiredAddons {
			if _, ok := c.Lookup(addon.Provider, addon.AddonName, target); !ok {
				missing = append(missing, fmt.Sprintf("%s: %s/%s", target, addon.Provider, addon.AddonName))
			}
		}
	}
	return missing
}

// StaleEntries returns every catalog entry whose LastVerifiedDate is
// before cutoff, in the same deterministic order Entries() already
// guarantees (provider, then add-on name, then target version) -- a
// report, never a validation failure on its own; see this catalog's
// Source Policy in docs/compatibility-catalog.md for why an old-but-still-
// accurate source must not be treated as broken.
func (c *Catalog) StaleEntries(cutoff time.Time) []Entry {
	if c == nil {
		return nil
	}
	var stale []Entry
	for _, entry := range c.Entries() {
		verified, err := time.Parse("2006-01-02", entry.LastVerifiedDate)
		if err != nil || verified.Before(cutoff) {
			stale = append(stale, entry)
		}
	}
	return stale
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
	if wantProvider, known := knownAddonProviders[entry.AddonName]; known && entry.Provider != wantProvider {
		return entry, fmt.Errorf("addonName %q must use provider %q, got %q", entry.AddonName, wantProvider, entry.Provider)
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
