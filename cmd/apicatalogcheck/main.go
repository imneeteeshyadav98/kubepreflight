// Command apicatalogcheck validates the embedded versioned API catalog
// (internal/apicatalog/versioned_catalog.json) and reports its coverage
// and staleness in a stable, human-readable form. Not part of the public
// CLI — not built or shipped by the Dockerfile, which only compiles
// ./cmd/kubepreflight. Run via scripts/check-api-version-catalog.sh,
// wired into CI's verify job so a broken, incomplete, or drifted catalog
// entry fails before merge, not silently at scan time.
//
// Exit codes:
//
//	0  catalog is valid, has full parity with the frozen legacy
//	   inventory, and covers its entire declared buildSupportedTargetRange
//	1  catalog failed schema/field validation, has drifted from the
//	   legacy inventory, or has a gap in its declared target-version range
//
// Staleness is report-only and never affects the exit code — see
// docs/api-version-catalog.md's Source Policy for why an old-but-still-
// accurate source must not be treated as broken.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/apicatalog"
)

func main() {
	staleAfterDays := flag.Int("stale-after-days", 180, "report catalog entries verified more than this many days ago; report only, never fails the command on its own")
	flag.Parse()

	catalog, err := apicatalog.DefaultVersioned()
	if err != nil {
		fmt.Fprintf(os.Stderr, "API version catalog failed validation: %v\n", err)
		os.Exit(1)
	}
	entries := catalog.Entries()
	fmt.Printf("API version catalog: schema and field validation OK (%d entries)\n\n", len(entries))

	printInventory(entries)

	ok := true

	issues, err := apicatalog.LegacyParityIssues()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nlegacy parity check failed to run: %v\n", err)
		os.Exit(1)
	}
	if len(issues) > 0 {
		fmt.Fprintln(os.Stderr, "\nFrozen legacy inventory parity: FAILED")
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "  - %s\n", issue)
		}
		ok = false
	} else {
		fmt.Println("\nFrozen legacy inventory parity: OK (every legacy GVK is present with matching fields)")
	}

	if !checkTargetRangeCoverage(catalog) {
		ok = false
	}

	printStaleness(catalog, *staleAfterDays)

	if !ok {
		os.Exit(1)
	}
}

func printInventory(entries []apicatalog.VersionedAPI) {
	fmt.Println("Group | Version | Kind | Deprecated | Removed | Supported target range | Verified")
	fmt.Println("------|---------|------|------------|---------|-------------------------|---------")
	for _, e := range entries {
		fmt.Printf("%s | %s | %s | %s | %s | %s-%s | %s\n",
			e.Group, e.Version, e.Kind, e.DeprecatedInVersion, e.RemovedInVersion,
			e.SupportedTargetRange.Min, e.SupportedTargetRange.Max, e.LastVerifiedDate)
	}
}

// checkTargetRangeCoverage walks the catalog's declared
// buildSupportedTargetRange one minor version at a time and confirms
// TargetSupported reports every one of them in range, and the versions
// immediately outside the range out of range. This is a defensive,
// explicit regression check on TargetSupported's own logic staying a
// single contiguous range rather than something that could develop a
// silent gap.
func checkTargetRangeCoverage(catalog *apicatalog.VersionedCatalog) bool {
	// TargetSupported always returns the declared buildSupportedTargetRange
	// bounds regardless of the probe value or its own ok result — "0.0" is
	// just a syntactically valid, certainly-out-of-range probe used purely
	// to read those bounds back out.
	min, max, _ := catalog.TargetSupported("0.0")
	minMajor, minMinor, err := parseMajorMinor(min)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nsupported target range coverage check failed: invalid declared min %q: %v\n", min, err)
		return false
	}
	maxMajor, maxMinor, err := parseMajorMinor(max)
	if err != nil || maxMajor != minMajor {
		fmt.Fprintf(os.Stderr, "\nsupported target range coverage check failed: invalid declared max %q: %v\n", max, err)
		return false
	}

	pass := true
	for minor := minMinor; minor <= maxMinor; minor++ {
		target := fmt.Sprintf("%d.%d", minMajor, minor)
		if _, _, ok := catalog.TargetSupported(target); !ok {
			fmt.Fprintf(os.Stderr, "supported target range coverage check failed: %s is inside the declared range %s-%s but TargetSupported reports it unsupported\n", target, min, max)
			pass = false
		}
	}
	belowRange := fmt.Sprintf("%d.%d", minMajor, minMinor-1)
	if _, _, ok := catalog.TargetSupported(belowRange); ok {
		fmt.Fprintf(os.Stderr, "supported target range coverage check failed: %s is below the declared range %s-%s but TargetSupported reports it supported\n", belowRange, min, max)
		pass = false
	}
	aboveRange := fmt.Sprintf("%d.%d", maxMajor, maxMinor+1)
	if _, _, ok := catalog.TargetSupported(aboveRange); ok {
		fmt.Fprintf(os.Stderr, "supported target range coverage check failed: %s is above the declared range %s-%s but TargetSupported reports it supported\n", aboveRange, min, max)
		pass = false
	}

	if pass {
		fmt.Printf("Supported target range coverage: OK (%s-%s fully covered; %s and %s correctly rejected)\n", min, max, belowRange, aboveRange)
	}
	return pass
}

// parseMajorMinor parses a bare "major.minor" Kubernetes version string —
// the buildSupportedTargetRange bounds TargetSupported already returns
// are always in this normalized form, so no leading "v" or patch
// component handling is needed here.
func parseMajorMinor(v string) (major, minor int, err error) {
	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid version %q", v)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid version %q: %w", v, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid version %q: %w", v, err)
	}
	return major, minor, nil
}

func printStaleness(c *apicatalog.VersionedCatalog, staleAfterDays int) {
	cutoff := time.Now().AddDate(0, 0, -staleAfterDays)
	stale := c.StaleEntries(cutoff)
	if len(stale) == 0 {
		fmt.Printf("\nNo catalog entries older than %d days.\n", staleAfterDays)
		return
	}
	fmt.Printf("\nCatalog entries older than %d days (report only, review recommended):\n", staleAfterDays)
	for _, e := range stale {
		fmt.Printf("  - %s/%s %s: last verified %s\n", e.Group, e.Version, e.Kind, e.LastVerifiedDate)
	}
}
