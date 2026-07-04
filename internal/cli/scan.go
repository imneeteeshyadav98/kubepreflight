package cli

import (
	"fmt"
	"io"
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

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/collectors/k8s"
	manifestcol "kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
	aksprovider "kubepreflight/internal/providers/aks"
	gkeprovider "kubepreflight/internal/providers/gke"
	"kubepreflight/internal/report"
	"kubepreflight/internal/reportserver"
	"kubepreflight/internal/rules"
)

func defaultKubeconfigPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

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
	var namespaceAllowlist []string
	var serveReport string
	var openReport bool
	var listenAddress string
	var terminalOutput string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan a cluster for EKS upgrade readiness risk",
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetVersion == "" {
				return fmt.Errorf("--target-version is required")
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
			terminalMode := effectiveTerminalOutput(terminalOutput, cmd.Flags().Changed("terminal-output"), serve)
			effectiveOutput := effectiveScanOutput(output, cmd.Flags().Changed("output"), serve)

			kubeConfigLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
				&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
			)

			restCfg, err := kubeConfigLoader.ClientConfig()
			if err != nil {
				return fmt.Errorf("loading kubeconfig: %w", err)
			}

			// --context wasn't set: resolve which context is actually in
			// use from kubeconfig so the report header names the real
			// cluster instead of showing a blank/dash.
			reportContext := kubeContext
			if reportContext == "" {
				if rawCfg, err := kubeConfigLoader.RawConfig(); err == nil {
					reportContext = rawCfg.CurrentContext
				}
			}

			clientset, err := kubernetes.NewForConfig(restCfg)
			if err != nil {
				return fmt.Errorf("building Kubernetes client: %w", err)
			}

			apiExtCli, err := apiextensionsclientset.NewForConfig(restCfg)
			if err != nil {
				return fmt.Errorf("building apiextensions client: %w", err)
			}

			dynamicClient, err := dynamic.NewForConfig(restCfg)
			if err != nil {
				return fmt.Errorf("building dynamic client: %w", err)
			}

			collector := k8s.NewCollector(clientset, apiExtCli, dynamicClient)
			snap, err := collector.Collect(cmd.Context())
			if err != nil {
				return fmt.Errorf("collecting cluster state: %w", err)
			}

			// AWS enrichment is opt-in (--provider=eks) and must never turn
			// into a hard failure of the whole scan: no credentials, no IAM
			// permissions, or no AWS setup at all is a perfectly normal way
			// to run this tool. Cluster-only checks always still run.
			var awsSnap *awscol.Snapshot
			if provider == "eks" {
				awsCollector, err := awscol.LoadCollector(cmd.Context(), clusterName)
				if err != nil {
					if terminalMode != "silent" {
						fmt.Fprintf(cmd.OutOrStdout(), "AWS enrichment skipped (%v) — continuing with cluster-only checks.\n", err)
					}
				} else {
					awsSnap, err = awsCollector.Collect(cmd.Context(), targetVersion)
					if err != nil {
						return fmt.Errorf("collecting AWS state: %w", err)
					}
				}
			}

			// Manifest scanning (Plane 1) is additive: raw YAML directories
			// and rendered Helm charts, scanned alongside whatever live
			// cluster/AWS data was already collected above. It doesn't (yet)
			// make the cluster connection optional — that's a separate CI/PR
			// "no cluster access" mode, not this pass.
			var manifestSnap *manifestcol.Snapshot
			if len(manifestDirs) > 0 || len(helmCharts) > 0 {
				var charts []manifestcol.HelmChart
				for _, p := range helmCharts {
					charts = append(charts, manifestcol.HelmChart{Path: p})
				}
				manifestCollector := manifestcol.NewCollector(manifestDirs, charts)
				manifestSnap, err = manifestCollector.Collect(cmd.Context())
				if err != nil {
					return fmt.Errorf("collecting manifest state: %w", err)
				}
			}

			sc := &rules.ScanContext{K8s: snap, AWS: awsSnap, Manifests: manifestSnap}

			registry := rules.NewDefaultRegistry()
			fs, err := registry.RunAll(sc, targetVersion)
			if err != nil {
				return fmt.Errorf("running rules: %w", err)
			}
			fs = findings.FilterByNamespaceAllowlist(fs, namespaceAllowlist)

			rpt := findings.NewReport(targetVersion, reportContext, provider, time.Now().UTC(), fs)
			rpt.NamespaceAllowlist = namespaceAllowlist
			*exitCode = rpt.ExitCode()

			// "Collected: ..." is collector-internal diagnostic detail (raw
			// object counts), not part of the compact summary's field list
			// — full mode only.
			if terminalMode == "full" {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Collected: %d nodes, %d pods, %d PDBs, %d webhooks, %d services, %d endpointslices, %d CRDs, %d deployments, %d daemonsets | AWS enrichment: %v | Findings: %d\n\n",
					len(snap.Nodes), len(snap.Pods), len(snap.PodDisruptionBudgets),
					len(snap.ValidatingWebhookConfigs)+len(snap.MutatingWebhookConfigs),
					len(snap.Services), len(snap.EndpointSlices), len(snap.CustomResourceDefinitions),
					len(snap.Deployments), len(snap.DaemonSets), awsSnap != nil, len(fs))
			}
			// Partial-scan notices are short and operationally significant
			// (they mean the report may be incomplete) — not the kind of
			// per-finding detail --terminal-output=compact exists to
			// suppress, so both full and compact print them; only silent
			// (errors only) drops them.
			if terminalMode != "silent" {
				if len(snap.Errors) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Partial cluster scan — collectors failed: %v\n", snap.Errors)
				}
				if awsSnap != nil && len(awsSnap.Errors) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Partial AWS scan — collectors failed: %v\n", awsSnap.Errors)
				}
				if manifestSnap != nil && len(manifestSnap.Errors) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Partial manifest scan — collectors failed: %v\n", manifestSnap.Errors)
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
			for _, target := range requestedReportTargets(effectiveOutput, findingsOut, serve) {
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
				if err := serveReports(cmd, findingsOut, listenAddress, openReport); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", defaultKubeconfigPath(), "path to kubeconfig")
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
	cmd.Flags().StringSliceVar(&namespaceAllowlist, "namespace-allowlist", nil, "only include namespaced findings from these namespaces (comma-separated or repeatable; cluster-scoped and AWS findings remain included)")
	cmd.Flags().StringVar(&serveReport, "serve-report", "auto", "serve generated reports locally: auto, always, or never")
	cmd.Flags().BoolVar(&openReport, "open-report", false, "open the local HTML report in the default browser (failure is non-fatal)")
	cmd.Flags().StringVar(&listenAddress, "listen", "127.0.0.1:0", "local report server listen address")
	cmd.Flags().StringVar(&terminalOutput, "terminal-output", "full", "stdout detail level: compact, full, or silent (default becomes compact when the local report server starts, unless set explicitly)")

	return cmd
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
	targets := []reportTarget{{path: findingsOut, write: report.WriteJSON}}
	if output == "md" || output == "all" {
		targets = append(targets, reportTarget{path: "report.md", write: report.WriteMarkdown})
	}
	if output == "html" || output == "all" || ensureHTML {
		targets = append(targets, reportTarget{path: "report.html", write: report.WriteHTML})
	}
	return targets
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

func serveReports(cmd *cobra.Command, findingsOut, listenAddress string, openReport bool) error {
	outputDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve report output directory: %w", err)
	}
	server, err := reportserver.Start(reportserver.Config{
		Listen: listenAddress, OutputDir: outputDir, FindingsPath: findingsOut,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nOpen report:\n  %s\n", server.ReportURL())
	if consoleURL, ok := server.ConsoleURL(); ok {
		fmt.Fprintf(cmd.OutOrStdout(), "\nOpen Console:\n  %s\n", consoleURL)
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

// writeReportFile creates path, renders rpt into it with the given writer
// function, and closes it — closing explicitly (not deferred) so a write
// error is never masked by a close error or vice versa.
func writeReportFile(path string, rpt *findings.Report, write func(*findings.Report, io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
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
