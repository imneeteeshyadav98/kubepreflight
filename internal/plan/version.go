// Package plan implements the multi-hop upgrade planner: given a cluster's
// current Kubernetes version and a further-out target, it generates the
// sequence of one-minor-version hops between them and assembles a
// PlanReport describing what's known for certain (the immediate next hop,
// scanned exactly like `scan` does) versus what's honestly only a
// prediction or requires a rescan once that hop is actually reached (see
// classify.go). This package holds pure logic only — no cluster/AWS I/O,
// no file writing — mirroring internal/rules' separation from its callers.
package plan

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseMajorMinor extracts the major and minor version numbers from a
// Kubernetes-style version string (e.g. "v1.33.0-eks-1234567" or "1.34"),
// ignoring any patch/build suffix. This is plan's own copy of the same
// parsing rules/node001.go implements privately — the two packages stay
// decoupled (internal/rules doesn't export it, and internal/plan must not
// import internal/rules just for this), matching how the wider codebase
// already keeps each package's helpers to itself unless reuse is needed
// across an actual dependency edge.
func ParseMajorMinor(v string) (major, minor int, err error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("cannot parse major.minor from %q", v)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing major version from %q: %w", v, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing minor version from %q: %w", v, err)
	}
	return major, minor, nil
}

// CompareMinor compares two same-major Kubernetes version strings by minor
// version, returning -1/0/1 (a<b, a==b, a>b). Cross-major comparisons
// return an error — EKS/Kubernetes upgrades are always single-major, so a
// major mismatch signals a caller bug or a malformed version string, not a
// meaningful ordering.
func CompareMinor(a, b string) (int, error) {
	aMajor, aMinor, err := ParseMajorMinor(a)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", a, err)
	}
	bMajor, bMinor, err := ParseMajorMinor(b)
	if err != nil {
		return 0, fmt.Errorf("parsing %q: %w", b, err)
	}
	if aMajor != bMajor {
		return 0, fmt.Errorf("cannot compare across major versions: %q vs %q", a, b)
	}
	switch {
	case aMinor < bMinor:
		return -1, nil
	case aMinor > bMinor:
		return 1, nil
	default:
		return 0, nil
	}
}

// Hop is one single-minor-version upgrade step, e.g. 1.29 -> 1.30. Index is
// 1-based; Index 1 is always the immediate next hop from the cluster's
// current version.
type Hop struct {
	Index int    `json:"index"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// GenerateHops builds the ordered list of one-minor-version hops from
// fromVersion (exclusive) to toVersion (inclusive). fromVersion and
// toVersion are normalized to "major.minor" form (patch/build suffixes are
// dropped) before being used as hop boundaries.
func GenerateHops(fromVersion, toVersion string) ([]Hop, error) {
	fromMajor, fromMinor, err := ParseMajorMinor(fromVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing --from-version %q: %w", fromVersion, err)
	}
	toMajor, toMinor, err := ParseMajorMinor(toVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing --to-version %q: %w", toVersion, err)
	}
	if fromMajor != toMajor {
		return nil, fmt.Errorf("cannot plan across major versions: %d.%d -> %d.%d", fromMajor, fromMinor, toMajor, toMinor)
	}
	if fromMinor == toMinor {
		return nil, fmt.Errorf("--from-version and --to-version are both %d.%d — nothing to plan", fromMajor, fromMinor)
	}
	if toMinor < fromMinor {
		return nil, fmt.Errorf("--to-version %d.%d is behind --from-version %d.%d — downgrades are not supported", toMajor, toMinor, fromMajor, fromMinor)
	}

	hops := make([]Hop, 0, toMinor-fromMinor)
	for i, minor := 1, fromMinor; minor < toMinor; i, minor = i+1, minor+1 {
		hops = append(hops, Hop{
			Index: i,
			From:  fmt.Sprintf("%d.%d", fromMajor, minor),
			To:    fmt.Sprintf("%d.%d", fromMajor, minor+1),
		})
	}
	return hops, nil
}
