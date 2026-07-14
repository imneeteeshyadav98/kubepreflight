// Command compatcatalogcheck validates the embedded compatibility catalog
// (internal/compatcatalog/catalog.json) and reports its coverage and
// staleness in a stable, human-readable form. Not part of the public
// CLI — not built or shipped by the Dockerfile, which only compiles
// ./cmd/kubepreflight. Run via scripts/check-compatibility-catalog.sh,
// wired into CI's verify job so a broken or incomplete catalog entry
// fails before merge, not silently at runtime as every affected add-on
// quietly falling back to "unverifiable".
//
// Exit codes:
//
//	0  catalog is valid and has full required coverage
//	1  catalog failed schema/field validation, or is missing required
//	   coverage for some catalog-supported target version
//
// Staleness is report-only and never affects the exit code — see
// docs/compatibility-catalog.md's Source Policy for why an old-but-still-
// accurate source must not be treated as broken.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"kubepreflight/internal/compatcatalog"
)

func main() {
	staleAfterDays := flag.Int("stale-after-days", 180, "report catalog entries verified more than this many days ago; report only, never fails the command on its own")
	flag.Parse()

	catalog, err := compatcatalog.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "compatibility catalog failed validation: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Compatibility catalog: schema and field validation OK")
	fmt.Println()

	printMatrix(catalog)

	missing := catalog.MissingRequiredEntries()
	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, "\nMissing required catalog coverage:")
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "  - %s\n", m)
		}
		os.Exit(1)
	}
	fmt.Println("\nRequired add-on coverage: OK (every catalog-supported target version covers every required add-on)")

	printStaleness(catalog, *staleAfterDays)
}

func printMatrix(c *compatcatalog.Catalog) {
	fmt.Println("Provider | Add-on | Kubernetes target | Minimum | Recommended | Verified")
	fmt.Println("---------|--------|--------------------|---------|-------------|---------")
	for _, e := range c.Entries() {
		fmt.Printf("%s | %s | %s | %s | %s | %s\n", e.Provider, e.AddonName, e.KubernetesVersion, e.MinimumCompatibleVersion, e.RecommendedVersion, e.LastVerifiedDate)
	}
}

func printStaleness(c *compatcatalog.Catalog, staleAfterDays int) {
	cutoff := time.Now().AddDate(0, 0, -staleAfterDays)
	stale := c.StaleEntries(cutoff)
	if len(stale) == 0 {
		fmt.Printf("\nNo catalog entries older than %d days.\n", staleAfterDays)
		return
	}
	fmt.Printf("\nCatalog entries older than %d days (report only, review recommended):\n", staleAfterDays)
	for _, e := range stale {
		fmt.Printf("  - %s/%s (target %s): last verified %s\n", e.Provider, e.AddonName, e.KubernetesVersion, e.LastVerifiedDate)
	}
}
