package cli

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/imneeteeshyadav98/kubepreflight/internal/apicatalog"
	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	manifestcol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/manifest"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/plan"
	aksprovider "github.com/imneeteeshyadav98/kubepreflight/internal/providers/aks"
	gkeprovider "github.com/imneeteeshyadav98/kubepreflight/internal/providers/gke"
	"github.com/imneeteeshyadav98/kubepreflight/internal/redact"
	"github.com/imneeteeshyadav98/kubepreflight/internal/report"
	"github.com/imneeteeshyadav98/kubepreflight/internal/reportserver"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
)

// validOutputs are the only accepted --output values. "all" writes every
// file format; a single value writes just that one. json stays the
// default for backward compatibility with existing scripts/tests.
var validOutputs = map[string]bool{"json": true, "md": true, "html": true, "all": true}
var validServeModes = map[string]bool{"auto": true, "always": true, "never": true}
var validTerminalOutputs = map[string]bool{"compact": true, "full": true, "silent": true}

func newScanCmd(exitCode *int) *cobra.Command {
	var kubeconfigPath string
	var kubeContext string
	var targetVersion string
	var output string
	var findingsOut string
	var provider string
	var clusterName string
	var resourceGroup string
	var subscriptionID string
	var gkeProject string
	var gkeLocation string
	var manifestDirs []string
	var helmCharts []string
	var manifestsOnly bool
	var namespaceAllowlist []string
	var serveReport string
	var openReport bool
	var listenAddress string
	var terminalOutput string
	var outputDir string
	var allowRemoteReport bool
	var collectorTimeout time.Duration
	var collectorConcurrency int
	var redactSensitiveIdentifiers bool

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan a cluster for Kubernetes upgrade readiness risk",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetVersion == "" {
				return fmt.Errorf("--target-version is required")
			}
			if _, _, err := plan.ParseMajorMinor(targetVersion); err != nil {
				return fmt.Errorf("--target-version %q is invalid: %w", targetVersion, err)
			}
			if err := rejectUnsupportedTargetVersion(targetVersion); err != nil {
				return err
			}
			if !validOutputs[output] {
				return fmt.Errorf("--output %q is not supported (use json, md, html, or all)", output)
			}
			if !validServeModes[serveReport] {
				return fmt.Errorf("--serve-report %q is not supported (use auto, always, or never)", serveReport)
			}
			if openReport && serveReport == "never" {
				return fmt.Errorf("--open-report cannot be used with --serve-report=never")
			}
			if !validTerminalOutputs[terminalOutput] {
				return fmt.Errorf("--terminal-output %q is not supported (use compact, full, or silent)", terminalOutput)
			}
			switch provider {
			case "", "eks":
				if provider == "eks" && clusterName == "" {
					return fmt.Errorf("--cluster-name is required when --provider=eks")
				}
			case "aks":
				if err := (aksprovider.Config{ClusterName: clusterName, ResourceGroup: resourceGroup, SubscriptionID: subscriptionID}).Validate(); err != nil {
					return err
				}
			case "gke":
				if err := (gkeprovider.Config{ClusterName: clusterName, Project: gkeProject, Location: gkeLocation}).Validate(); err != nil {
					return err
				}
			default:
				return fmt.Errorf("--provider %q is not supported (use eks, aks, gke, or omit for a cluster-only scan)", provider)
			}
			if provider == "aks" || provider == "gke" {
				return fmt.Errorf("--provider=%s is recognized but enrichment isn't implemented yet — cluster-only checks aren't run automatically for it in this command; see docs/provider-roadmap.md. Use --provider=eks or omit --provider today.", provider)
			}
			if manifestsOnly {
				if len(manifestDirs) == 0 && len(helmCharts) == 0 {
					return fmt.Errorf("--manifests-only requires --manifests or --helm-chart (nothing to scan otherwise)")
				}
				if provider != "" {
					return fmt.Errorf("--manifests-only cannot be combined with --provider (it skips all cluster and cloud-provider access)")
				}
				if kubeconfigPath != "" || kubeContext != "" {
					return fmt.Errorf("--manifests-only cannot be combined with --kubeconfig/--context (it never loads a kubeconfig)")
				}
			}
			if err := validateCollectorConcurrency(collectorConcurrency); err != nil {
				return err
			}
			var err error
			namespaceAllowlist, err = normalizeNamespaceAllowlist(namespaceAllowlist)
			if err != nil {
				return err
			}

			// Computed up front (before any cluster work) since neither
			// depends on collected data: --terminal-output's default only
			// switches to "compact" once we already know a local server is
			// about to start, matching the whole reason for shrinking
			// stdout — report.html/Console cover the detail instead.
			serve := shouldServeReport(serveReport, output, cmd.Flags().Changed("output"), writerIsTerminal(cmd.OutOrStdout()), os.Getenv("CI") != "")
			if openReport {
				serve = true
			}
			if serve {
				if err := validateListenAddress(listenAddress, allowRemoteReport); err != nil {
					return err
				}
			}
			terminalMode := effectiveTerminalOutput(terminalOutput, cmd.Flags().Changed("terminal-output"), serve)
			effectiveOutput := effectiveScanOutput(output, cmd.Flags().Changed("output"), serve)
			findingsPath := resolveOutputPath(outputDir, findingsOut)

			// Collection (Kubernetes, AWS, manifests) runs under a
			// Ctrl+C/SIGTERM-aware context, separate from the unadorned
			// cmd.Context() the post-scan "serving reports" phase uses
			// below (serveReports installs its own signal.NotifyContext
			// for that phase). Without this, interrupting a hung collector
			// call would do nothing until its own --collector-timeout
			// budget expired on its own.
			collectCtx, stopCollectSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stopCollectSignals()

			// --manifests-only skips kubeconfig loading and all cluster/AWS
			// collection entirely, so a PR-time manifest scan needs zero
			// credentials of any kind — see the --manifests-only flag
			// definition below for the validation that guards this.
			var reportContext string
			var currentVersion string
			var snap *k8s.Snapshot
			var awsSnap *awscol.Snapshot
			var awsUnavailable error
			if !manifestsOnly {
				loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
				if kubeconfigPath != "" {
					loadingRules.ExplicitPath = kubeconfigPath
				}
				kubeConfigLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
					loadingRules,
					&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
				)

				restCfg, err := kubeConfigLoader.ClientConfig()
				if err != nil {
					return infraFailure(fmt.Errorf("loading kubeconfig: %w", err))
				}
				// Explicit, documented rate limit rather than client-go's
				// own unset-default (QPS 5, Burst 10) -- see
				// k8s.DefaultClientQPS/DefaultClientBurst's doc comment for
				// why bounded --collector-concurrency needs this headroom.
				restCfg.QPS = k8s.DefaultClientQPS
				restCfg.Burst = k8s.DefaultClientBurst

				// --context wasn't set: resolve which context is actually in
				// use from kubeconfig so the report header names the real
				// cluster instead of showing a blank/dash.
				reportContext = kubeContext
				if reportContext == "" {
					if rawCfg, err := kubeConfigLoader.RawConfig(); err == nil {
						reportContext = rawCfg.CurrentContext
					}
				}

				clientset, err := kubernetes.NewForConfig(restCfg)
				if err != nil {
					return infraFailure(fmt.Errorf("building Kubernetes client: %w", err))
				}

				apiExtCli, err := apiextensionsclientset.NewForConfig(restCfg)
				if err != nil {
					return infraFailure(fmt.Errorf("building apiextensions client: %w", err))
				}

				dynamicClient, err := dynamic.NewForConfig(restCfg)
				if err != nil {
					return infraFailure(fmt.Errorf("building dynamic client: %w", err))
				}

				collector := k8s.NewCollector(clientset, apiExtCli, dynamicClient)
				if serverVersion, versionErr := collector.ServerVersion(collectCtx, collectorTimeout); versionErr == nil {
					if normalized, ok := findings.NormalizeKubernetesVersion(serverVersion); ok {
						currentVersion = normalized
					}
				}
				if err := rejectDowngrade(currentVersion, targetVersion); err != nil {
					return err
				}
				snap, err = collector.Collect(collectCtx, collectorTimeout, collectorConcurrency)
				if err != nil {
					return infraFailure(fmt.Errorf("collecting cluster state: %w", err))
				}

				// AWS enrichment is opt-in (--provider=eks) and must never
				// turn into a hard failure of the whole scan: no
				// credentials, no IAM permissions, or no AWS setup at all
				// is a perfectly normal way to run this tool. Cluster-only
				// checks always still run.
				if provider == "eks" {
					awsCollector, err := awscol.LoadCollector(collectCtx, clusterName)
					if err != nil {
						awsUnavailable = err
						if terminalMode != "silent" {
							fmt.Fprintf(cmd.OutOrStdout(), "AWS enrichment skipped (%v) — continuing with cluster-only checks.\n", err)
						}
					} else {
						awsSnap, err = awsCollector.Collect(collectCtx, collectorTimeout, targetVersion)
						if err != nil {
							return fmt.Errorf("collecting AWS state: %w", err)
						}
					}
				}
			}

			// Manifest scanning (Plane 1) is additive: raw YAML directories
			// and rendered Helm charts, scanned alongside whatever live
			// cluster/AWS data was already collected above (or on their
			// own, with --manifests-only).
			var manifestSnap *manifestcol.Snapshot
			if len(manifestDirs) > 0 || len(helmCharts) > 0 {
				var charts []manifestcol.HelmChart
				for _, p := range helmCharts {
					charts = append(charts, manifestcol.HelmChart{Path: p})
				}
				manifestCollector := manifestcol.NewCollector(manifestDirs, charts)
				manifestSnap, err = manifestCollector.Collect(collectCtx, collectorTimeout)
				if err != nil {
					return fmt.Errorf("collecting manifest state: %w", err)
				}
			}

			sc := &rules.ScanContext{K8s: snap, AWS: awsSnap, Manifests: manifestSnap}

			registry := rules.NewDefaultRegistry()
			if manifestsOnly {
				registry = rules.NewManifestsOnlyRegistry()
			}
			fs, err := registry.RunAll(sc, targetVersion)
			if err != nil {
				return fmt.Errorf("running rules: %w", err)
			}
			fs = findings.FilterByNamespaceAllowlist(fs, namespaceAllowlist)

			rpt := findings.NewReport(targetVersion, reportContext, provider, time.Now().UTC(), fs)
			rpt.CurrentVersion = currentVersion
			rpt.NamespaceAllowlist = namespaceAllowlist
			rpt.SetCoverage(buildScanCoverage(snap, awsSnap, manifestSnap, !manifestsOnly, provider == "eks", len(manifestDirs) > 0 || len(helmCharts) > 0, awsUnavailable))
			rpt.EKSCluster = eksClusterInfo(clusterName, awsSnap)
			rpt.EKSAddons = eksAddonInfos(awsSnap)
			rpt.EKSNodegroups = eksNodegroupInfos(awsSnap)
			rpt.EKSUpgradeInsights = eksUpgradeInsightInfos(awsSnap)
			// Every field is set by this point; redact before any
			// report.Write* call below. ExitCode() depends only on
			// Coverage/Findings.Severity, never on a redacted string field,
			// so this is safe on either side of the next line.
			if redactSensitiveIdentifiers {
				redact.Report(rpt)
			}
			*exitCode = rpt.ExitCode()

			// "Collected: ..." is collector-internal diagnostic detail (raw
			// object counts), not part of the compact summary's field list
			// — full mode only.
			if terminalMode == "full" && snap != nil {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Collected: %d nodes, %d pods, %d PDBs, %d webhooks, %d services, %d endpointslices, %d CRDs, %d deployments, %d daemonsets | AWS enrichment: %v | Findings: %d\n\n",
					len(snap.Nodes), len(snap.Pods), len(snap.PodDisruptionBudgets),
					len(snap.ValidatingWebhookConfigs)+len(snap.MutatingWebhookConfigs),
					len(snap.Services), len(snap.EndpointSlices), len(snap.CustomResourceDefinitions),
					len(snap.Deployments), len(snap.DaemonSets), awsSnap != nil, len(fs))
			} else if terminalMode == "full" {
				fmt.Fprintf(cmd.OutOrStdout(), "Collected: --manifests-only, no cluster/AWS collection | Findings: %d\n\n", len(fs))
			}
			// Partial-scan notices are short and operationally significant
			// (they mean the report may be incomplete) — not the kind of
			// per-finding detail --terminal-output=compact exists to
			// suppress, so both full and compact print them; only silent
			// (errors only) drops them.
			if terminalMode != "silent" {
				if snap != nil {
					writePartialScanNotice(cmd.OutOrStdout(), "cluster", snap.Errors)
				}
				if awsSnap != nil {
					writePartialScanNotice(cmd.OutOrStdout(), "AWS", awsSnap.Errors)
				}
				if manifestSnap != nil {
					writePartialScanNotice(cmd.OutOrStdout(), "manifest", manifestSnap.Errors)
				}
			}

			switch terminalMode {
			case "full":
				if err := report.WriteTerminal(rpt, cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("rendering terminal report: %w", err)
				}
			case "compact":
				if err := report.WriteCompactSummary(rpt, cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("rendering terminal summary: %w", err)
				}
			case "silent":
				// Nothing on success — report.html/findings.json/Console
				// carry the detail; serveReports below still prints the
				// URLs if a server is starting.
			}

			var writtenFiles []string
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			for _, target := range requestedReportTargetsInDir(effectiveOutput, findingsPath, serve, outputDir) {
				if err := writeReportFile(target.path, rpt, target.write); err != nil {
					return err
				}
				writtenFiles = append(writtenFiles, target.path)
			}

			if terminalMode != "silent" {
				fmt.Fprintln(cmd.OutOrStdout(), "\nReports written:")
				for _, path := range writtenFiles {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", path)
				}
			}
			if serve {
				if err := serveReports(cmd, findingsPath, outputDir, listenAddress, !cmd.Flags().Changed("listen"), openReport, terminalMode, false, allowRemoteReport); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "path to kubeconfig (defaults to standard KUBECONFIG/home loading rules)")
	cmd.Flags().StringVar(&kubeContext, "context", "", "kubeconfig context to use")
	cmd.Flags().StringVar(&targetVersion, "target-version", "", "target Kubernetes version, e.g. 1.34 (required)")
	cmd.Flags().StringVar(&output, "output", "json", "output format: json, md, html, or all")
	cmd.Flags().StringVar(&findingsOut, "findings-out", "findings.json", "path to canonical JSON findings (always written, regardless of --output)")
	cmd.Flags().StringVar(&provider, "provider", "", "cloud provider for enrichment checks: eks, aks, gke, or omit for a cluster-only scan (aks/gke are recognized but not implemented yet — see docs/provider-roadmap.md)")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "EKS/AKS/GKE cluster name (required when --provider is set)")
	cmd.Flags().StringVar(&resourceGroup, "resource-group", "", "Azure resource group (required when --provider=aks)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (optional; falls back to az/env default when --provider=aks)")
	cmd.Flags().StringVar(&gkeProject, "project", "", "GCP project ID (required when --provider=gke)")
	cmd.Flags().StringVar(&gkeLocation, "location", "", "GCP zone or region (required when --provider=gke)")
	cmd.Flags().StringArrayVar(&manifestDirs, "manifests", nil, "directory of raw YAML manifests to scan for deprecated APIs (repeatable)")
	cmd.Flags().StringArrayVar(&helmCharts, "helm-chart", nil, "path to a Helm chart to render (via helm template) and scan for deprecated APIs (repeatable)")
	cmd.Flags().BoolVar(&manifestsOnly, "manifests-only", false, "skip kubeconfig loading and all cluster/AWS collection entirely; scan only --manifests/--helm-chart (requires at least one of those, and cannot be combined with --provider or --kubeconfig/--context)")
	cmd.Flags().StringSliceVar(&namespaceAllowlist, "namespace-allowlist", nil, "only include namespaced findings from these namespaces (comma-separated or repeatable; cluster-scoped and AWS findings remain included)")
	cmd.Flags().StringVar(&serveReport, "serve-report", "auto", "serve generated reports locally: auto, always, or never")
	cmd.Flags().BoolVar(&openReport, "open-report", false, "open the local HTML report in the default browser (failure is non-fatal)")
	cmd.Flags().StringVar(&listenAddress, "listen", "127.0.0.1:8080", "local report server listen address (falls back to a random free port if this one is busy, unless explicitly set)")
	cmd.Flags().StringVar(&terminalOutput, "terminal-output", "full", "stdout detail level: compact, full, or silent (default becomes compact when the local report server starts, unless set explicitly)")
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "directory for generated report artifacts")
	cmd.Flags().BoolVar(&allowRemoteReport, "allow-remote-report", false, "allow serving unauthenticated reports on a non-loopback address")
	cmd.Flags().BoolVar(&redactSensitiveIdentifiers, "redact-sensitive-identifiers", false, "replace AWS ARNs and EC2-style internal node hostnames with placeholders in every output (findings.json, report.md/html, terminal) — use before sharing generated evidence outside your organization; does not change findings, scores, or exit codes")
	cmd.Flags().DurationVar(&collectorTimeout, "collector-timeout", k8s.DefaultCollectorTimeout, "per-call (not per-scan) timeout for each Kubernetes, AWS, and Helm-chart-render collector request (e.g. 45s, 2m); a timed-out call is recorded like any other collection failure and marks that plane's coverage partial -- against a fully unreachable cluster/AWS endpoint, total worst-case wait is roughly (number of calls) x this value, since each call gets its own budget")
	cmd.Flags().IntVar(&collectorConcurrency, "collector-concurrency", k8s.DefaultCollectorConcurrency, "maximum number of Kubernetes collector requests in flight at once (1-16); 1 preserves fully sequential collection, higher values reduce wall-clock time on large clusters at the cost of more simultaneous load on the API server (bounded by an explicit, conservative client-side QPS/Burst limit regardless of this value)")

	return cmd
}

func writePartialScanNotice(w io.Writer, plane string, errs map[string]error) {
	if len(errs) == 0 {
		return
	}
	fmt.Fprintf(w, "Partial %s scan — collectors failed:\n", plane)
	for _, line := range stableErrors(coveragePlaneForNotice(plane), errs) {
		fmt.Fprintf(w, "  - %s\n", line)
	}
}

func coveragePlaneForNotice(plane string) string {
	switch plane {
	case "cluster":
		return "kubernetes"
	case "AWS":
		return "aws"
	case "manifest":
		return "manifests"
	default:
		return plane
	}
}

// effectiveTerminalOutput mirrors effectiveScanOutput's pattern: an
// explicit --terminal-output always wins. Left unset, the flag's own
// default ("full") only gets overridden to "compact" once we already know
// a local server is starting — report.html/Console cover the per-finding
// detail then, so stdout doesn't need to repeat it. Non-serving runs
// (scripts, CI, --serve-report=never) keep today's full terminal output
// untouched.
func effectiveTerminalOutput(mode string, explicit, serve bool) string {
	if !explicit && serve {
		return "compact"
	}
	return mode
}

type reportTarget struct {
	path  string
	write func(*findings.Report, io.Writer) error
}

// requestedReportTargets always includes canonical JSON. --output selects the
// additional human-readable artifact, rather than disabling the machine-
// readable findings contract CI callers rely on.
func requestedReportTargets(output, findingsOut string, ensureHTML bool) []reportTarget {
	return requestedReportTargetsInDir(output, findingsOut, ensureHTML, ".")
}

func requestedReportTargetsInDir(output, findingsOut string, ensureHTML bool, outputDir string) []reportTarget {
	targets := []reportTarget{{path: findingsOut, write: report.WriteJSON}}
	if output == "md" || output == "all" {
		targets = append(targets, reportTarget{path: filepath.Join(outputDir, "report.md"), write: report.WriteMarkdown})
	}
	if output == "html" || output == "all" || ensureHTML {
		targets = append(targets, reportTarget{path: filepath.Join(outputDir, "report.html"), write: report.WriteHTML})
	}
	return targets
}

func resolveOutputPath(outputDir, path string) string {
	if filepath.IsAbs(path) || filepath.Dir(path) != "." {
		return path
	}
	return filepath.Join(outputDir, path)
}

func shouldServeReport(mode, output string, outputExplicit, interactive, ci bool) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	default:
		return interactive && !ci && !(outputExplicit && output == "json")
	}
}

func effectiveScanOutput(output string, outputExplicit, serve bool) string {
	if serve && !outputExplicit {
		return "all"
	}
	return output
}

func writerIsTerminal(w io.Writer) bool {
	type fdWriter interface{ Fd() uintptr }
	fd, ok := w.(fdWriter)
	return ok && term.IsTerminal(int(fd.Fd()))
}

func serveReports(cmd *cobra.Command, findingsOut, outputDir, listenAddress string, listenFallbackOnBusy, openReport bool, terminalMode string, includePlan bool, allowRemoteReport bool) error {
	absFindings, err := filepath.Abs(findingsOut)
	if err != nil {
		return fmt.Errorf("resolve findings path: %w", err)
	}
	server, err := reportserver.Start(reportserver.Config{
		Listen: listenAddress, OutputDir: outputDir, FindingsPath: absFindings,
		FallbackOnBusy: listenFallbackOnBusy, ServePlan: includePlan,
	})
	if err != nil {
		return err
	}

	// validateListenAddress already refused a non-loopback --listen unless
	// allowRemoteReport was passed, so this can only be reached with a
	// loopback address OR an explicit opt-in — but the opt-in itself still
	// deserves a persistent, hard-to-miss reminder every time the server
	// actually starts, not just the one-time flag-parse gate that's long
	// scrolled off by the time someone's looking at the printed URL.
	if allowRemoteReport || !isLoopbackAddress(listenAddress) {
		fmt.Fprintf(cmd.OutOrStdout(), "\n⚠ WARNING: report server is exposed beyond loopback (--listen %s, --allow-remote-report) and is UNAUTHENTICATED.\n", listenAddress)
		fmt.Fprintln(cmd.OutOrStdout(), "  Anyone who can reach this address can read this scan's findings — including namespaces, resource UIDs, manifest paths, and cloud (AWS) IDs.")
	}

	// Only one primary URL by default — report.html now links to the
	// Console itself (see html.go's "Open Interactive Console" button).
	// Full/verbose mode also prints the Console URL directly, since that
	// mode is for users who want every detail on stdout.
	fmt.Fprintf(cmd.OutOrStdout(), "\nOpen KubePreflight report:\n  %s\n", server.ReportURL())
	if terminalMode == "full" {
		if consoleURL, ok := server.ConsoleURL(); ok {
			fmt.Fprintf(cmd.OutOrStdout(), "\nOpen Console:\n  %s\n", consoleURL)
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\nPress Ctrl+C to stop serving reports.")
	if openReport {
		if err := reportserver.OpenBrowser(server.ReportURL()); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Could not open report browser (%v); server is still running.\n", err)
		}
	}

	signalCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Wait(signalCtx); err != nil {
		return err
	}
	return nil
}

// unsupportedDowngradeError builds the shared, canonical error both scan
// and plan return when the requested target represents a downgrade from
// the detected current version -- a downgrade is not a supported
// operation this tool assesses (see findings.CompareMinorVersions), so
// this fails fast rather than running a scan whose findings would imply
// a downgrade is something to remediate toward. current and target are
// normalized to plain "major.minor" for display -- plan's resolved
// "from" version in particular can be a raw server string like
// "v1.32.2-eks-1234567" (unlike scan's, which is already normalized
// before this is called), and this error must read identically from
// either caller. Falls back to the raw string only if normalization
// somehow fails, which callers of rejectDowngrade never actually hit
// (it only calls this after CompareMinorVersions already parsed both
// successfully) -- kept only so this function has no way to silently
// drop version information it was given.
func unsupportedDowngradeError(current, target string) error {
	if normalized, ok := findings.NormalizeKubernetesVersion(current); ok {
		current = normalized
	}
	if normalized, ok := findings.NormalizeKubernetesVersion(target); ok {
		target = normalized
	}
	return fmt.Errorf("downgrade is not supported: current Kubernetes version is %s, target version is %s. Choose a target version greater than %s.", current, target, current)
}

// rejectDowngrade is the one shared guard scan and plan both call, right
// after the current Kubernetes version becomes known and before any
// further collection, report generation, or action-plan construction.
// Returns nil (proceed normally) whenever current is empty or either
// version fails to parse -- version detection can fail (no permissions,
// unreachable API server) or never run at all (--manifests-only), and
// this must never guess "downgrade" without being sure, matching the
// "don't guess" principle findings.Report.UpgradeApplicable already
// applies to the same-version case. Cross-major comparisons (which
// findings.CompareMinorVersions treats as an error, not a direction) are
// likewise left alone here for the same reason -- an ambiguous pair is
// not a confidently-known downgrade.
func rejectDowngrade(current, target string) error {
	if current == "" {
		return nil
	}
	relation, err := findings.CompareMinorVersions(current, target)
	if err != nil || relation != findings.VersionDowngrade {
		return nil
	}
	return unsupportedDowngradeError(current, target)
}

// unsupportedTargetVersionError builds the shared, canonical error scan and
// plan both return when the requested target falls outside this build's
// declared apicatalog.VersionedCatalog coverage (see
// apicatalog.VersionedCatalog.TargetSupported). min/max are that catalog's
// normalized declared bounds.
func unsupportedTargetVersionError(target, min, max string) error {
	if normalized, ok := findings.NormalizeKubernetesVersion(target); ok {
		target = normalized
	}
	return fmt.Errorf("target Kubernetes version %s is not supported by this KubePreflight build. Supported target versions: %s–%s. Upgrade KubePreflight or choose a supported target version.", target, min, max)
}

// rejectUnsupportedTargetVersion is the one shared guard scan and plan both
// call, right after their own --target-version/--to-version syntax check
// and before any kubeconfig loading or collection -- it needs nothing but
// the target string, so it fails as fast as a bad flag value can. The
// versioned API catalog declares the Kubernetes target-version range this
// build's removed/deprecated-API data has actually been reviewed against;
// a target outside that range must not silently reach collectors and
// rules whose underlying facts were never verified for it. This is a
// coarser, build-wide check than any single apicatalog.VersionedAPI
// entry's own SupportedTargetRange (see internal/rules/api_catalog.go's
// per-entry fallback, which stays in place unchanged for rule-level
// decisions once a scan is actually allowed to run).
func rejectUnsupportedTargetVersion(target string) error {
	vc, err := apicatalog.DefaultVersioned()
	if err != nil {
		return fmt.Errorf("loading API version catalog: %w", err)
	}
	min, max, ok := vc.TargetSupported(target)
	if ok {
		return nil
	}
	return unsupportedTargetVersionError(target, min, max)
}

// validateCollectorConcurrency rejects an out-of-range --collector-concurrency
// before any collection starts, rather than silently clamping it (which
// k8s.Collector.Collect itself also does, defensively, but a CLI flag typo
// like --collector-concurrency 0 or 200 deserves an explicit error, not a
// silently different value than what was asked for). Shared by scan and
// plan since both expose the same flag with the same bounds.
func validateCollectorConcurrency(concurrency int) error {
	if concurrency < k8s.MinCollectorConcurrency || concurrency > k8s.MaxCollectorConcurrency {
		return fmt.Errorf("--collector-concurrency must be between %d and %d, got %d", k8s.MinCollectorConcurrency, k8s.MaxCollectorConcurrency, concurrency)
	}
	return nil
}

func validateListenAddress(address string, allowRemote bool) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid --listen address %q: %w", address, err)
	}
	if allowRemote || isLoopbackHost(host) {
		return nil
	}
	return fmt.Errorf("--listen %q is not loopback-only; pass --allow-remote-report to acknowledge that reports are unauthenticated", address)
}

// isLoopbackAddress reports whether a "host:port" listen address resolves
// to loopback. Used only to decide whether the non-loopback exposure
// warning should print — an unparseable address here would already have
// been rejected by validateListenAddress before serveReports ever runs, so
// this treats a parse failure as "not loopback" (favoring the warning)
// rather than erroring a second time.
func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	return isLoopbackHost(host)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizeNamespaceAllowlist(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	var out []string
	for _, value := range values {
		namespace := strings.TrimSpace(value)
		if namespace == "" {
			return nil, fmt.Errorf("--namespace-allowlist contains an empty namespace")
		}
		if problems := k8svalidation.IsDNS1123Label(namespace); len(problems) > 0 {
			return nil, fmt.Errorf("invalid namespace %q in --namespace-allowlist: %s", namespace, strings.Join(problems, "; "))
		}
		if !seen[namespace] {
			seen[namespace] = true
			out = append(out, namespace)
		}
	}
	sort.Strings(out)
	return out, nil
}

// createReportFile opens path for a fresh report write, refusing to follow
// a pre-existing symlink. In a shared output directory (a multi-tenant CI
// workspace, a shared /tmp subpath), a same-user attacker could otherwise
// pre-place a symlink at the report's target filename pointing at a file
// the victim never intended to touch — O_TRUNC would then silently
// truncate/overwrite whatever the symlink points at. This only refuses
// when path itself already exists as a symlink; writing a new file, or
// overwriting an existing regular file, is unaffected.
func createReportFile(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to write %s: existing path is a symlink", path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("creating %s: %w", path, err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, fmt.Errorf("securing %s: %w", path, err)
	}
	return f, nil
}

// writeReportFile creates path, renders rpt into it with the given writer
// function, and closes it — closing explicitly (not deferred) so a write
// error is never masked by a close error or vice versa.
func writeReportFile(path string, rpt *findings.Report, write func(*findings.Report, io.Writer) error) error {
	f, err := createReportFile(path)
	if err != nil {
		return err
	}
	if err := write(rpt, f); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}
	return nil
}
