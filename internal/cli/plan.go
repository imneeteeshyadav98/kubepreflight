package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/collectors/k8s"
	manifestcol "kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/plan"
	aksprovider "kubepreflight/internal/providers/aks"
	gkeprovider "kubepreflight/internal/providers/gke"
	"kubepreflight/internal/report"
	"kubepreflight/internal/rules"
)

// manifestProjectableRules maps a rule ID with plan.ProjectFromManifests
// policy to the Rule instance that re-evaluates it for a future hop, off
// the same manifest/CRD snapshot already collected for hop 1 (no
// re-collection — a YAML file's apiVersion doesn't change hop to hop).
var manifestProjectableRules = map[string]rules.Rule{
	"API-001": rules.API001{},
}

// awsProjectableRules maps a rule ID with plan.ProjectFromFreshAWSQuery
// policy to the Rule instance that re-evaluates it for a future hop,
// against a freshly re-collected AWS snapshot for that hop's target
// version (only when --provider=eks and AWS enrichment loaded for hop 1).
var awsProjectableRules = map[string]rules.Rule{
	"API-002":   rules.API002{},
	"ADDON-001": rules.ADDON001{},
}

func newPlanCmd(exitCode *int) *cobra.Command {
	var kubeconfigPath string
	var kubeContext string
	var fromVersion string
	var toVersion string
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
		Use:   "plan",
		Short: "Plan a multi-hop EKS upgrade path and scan the immediate next hop",
		RunE: func(cmd *cobra.Command, args []string) error {
			if toVersion == "" {
				return fmt.Errorf("--to-version is required")
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
				return fmt.Errorf("--provider %q is not supported (use eks, aks, gke, or omit for a cluster-only plan)", provider)
			}
			if provider == "aks" || provider == "gke" {
				return fmt.Errorf("--provider=%s is recognized but enrichment isn't implemented yet — cluster-only checks aren't run automatically for it in this command; see docs/provider-roadmap.md. Use --provider=eks or omit --provider today.", provider)
			}
			if _, _, err := plan.ParseMajorMinor(toVersion); err != nil {
				return fmt.Errorf("--to-version %q is invalid: %w", toVersion, err)
			}
			if fromVersion != "" && fromVersion != "auto" {
				if _, _, err := plan.ParseMajorMinor(fromVersion); err != nil {
					return fmt.Errorf("--from-version %q is invalid: %w", fromVersion, err)
				}
			}
			var err error
			namespaceAllowlist, err = normalizeNamespaceAllowlist(namespaceAllowlist)
			if err != nil {
				return err
			}

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

			k8sCollector := k8s.NewCollector(clientset, apiExtCli, dynamicClient)

			resolvedFromVersion, fromVersionSource, err := resolveFromVersion(cmd.Context(), fromVersion, provider, clusterName, k8sCollector, cmd.OutOrStdout(), terminalMode)
			if err != nil {
				return err
			}

			hops, err := plan.GenerateHops(resolvedFromVersion, toVersion)
			if err != nil {
				return err
			}

			snap, err := k8sCollector.Collect(cmd.Context())
			if err != nil {
				return fmt.Errorf("collecting cluster state: %w", err)
			}

			// AWS enrichment is opt-in (--provider=eks) and must never turn
			// into a hard failure — same graceful-skip pattern as `scan`.
			var awsSnap *awscol.Snapshot
			var awsCollector *awscol.Collector
			if provider == "eks" {
				var loadErr error
				awsCollector, loadErr = awscol.LoadCollector(cmd.Context(), clusterName)
				if loadErr != nil {
					if terminalMode != "silent" {
						fmt.Fprintf(cmd.OutOrStdout(), "AWS enrichment skipped (%v) — continuing with cluster-only checks.\n", loadErr)
					}
					awsCollector = nil
				} else {
					awsSnap, err = awsCollector.Collect(cmd.Context(), hops[0].To)
					if err != nil {
						return fmt.Errorf("collecting AWS state: %w", err)
					}
				}
			}

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

			// Hop 1: byte-for-byte the same sequence `scan` runs — a real,
			// exact scan. Nothing about `plan` changes what happens for the
			// immediate next hop.
			sc := &rules.ScanContext{K8s: snap, AWS: awsSnap, Manifests: manifestSnap}
			registry := rules.NewDefaultRegistry()
			fs, err := registry.RunAll(sc, hops[0].To)
			if err != nil {
				return fmt.Errorf("running rules: %w", err)
			}
			fs = findings.FilterByNamespaceAllowlist(fs, namespaceAllowlist)

			hop1Report := findings.NewReport(hops[0].To, reportContext, provider, time.Now().UTC(), fs)
			hop1Report.NamespaceAllowlist = namespaceAllowlist
			*exitCode = hop1Report.ExitCode()

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

			assessFutureHop := func(hop plan.Hop) (plan.HopReport, error) {
				return assessHop(cmd.Context(), hop, sc, reportContext, provider, awsCollector,
					buildRecommendedScanCommand(hop.To, provider, clusterName, manifestDirs, helmCharts, namespaceAllowlist))
			}

			planReport, err := plan.BuildPlan(reportContext, provider, resolvedFromVersion, fromVersionSource, toVersion, hops, hop1Report, assessFutureHop, time.Now().UTC())
			if err != nil {
				return fmt.Errorf("building plan: %w", err)
			}

			switch terminalMode {
			case "full":
				if err := report.WriteTerminal(hop1Report, cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("rendering terminal report: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout())
				if err := report.WritePlanCompactSummary(planReport, cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("rendering plan path summary: %w", err)
				}
			case "compact":
				if err := report.WritePlanCompactSummary(planReport, cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("rendering plan summary: %w", err)
				}
			case "silent":
				// Nothing on success — upgrade-plan.json/report.html/Console
				// carry the detail.
			}

			var writtenFiles []string
			for _, target := range requestedReportTargets(effectiveOutput, findingsOut, serve) {
				if err := writeReportFile(target.path, hop1Report, target.write); err != nil {
					return err
				}
				writtenFiles = append(writtenFiles, target.path)
			}

			const planPath = "upgrade-plan.json"
			if err := writePlanReportFile(planPath, planReport); err != nil {
				return err
			}
			writtenFiles = append(writtenFiles, planPath)

			if terminalMode != "silent" {
				fmt.Fprintln(cmd.OutOrStdout(), "\nReports written:")
				for _, path := range writtenFiles {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", path)
				}
			}
			if serve {
				if err := serveReports(cmd, findingsOut, listenAddress, !cmd.Flags().Changed("listen"), openReport, terminalMode); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", defaultKubeconfigPath(), "path to kubeconfig")
	cmd.Flags().StringVar(&kubeContext, "context", "", "kubeconfig context to use")
	cmd.Flags().StringVar(&fromVersion, "from-version", "auto", "current Kubernetes version to plan from; \"auto\" detects it via EKS DescribeCluster (--provider=eks) or the cluster's server version")
	cmd.Flags().StringVar(&toVersion, "to-version", "", "target Kubernetes version for the end of the upgrade path, e.g. 1.36 (required)")
	cmd.Flags().StringVar(&output, "output", "json", "output format for the immediate next hop: json, md, html, or all")
	cmd.Flags().StringVar(&findingsOut, "findings-out", "findings.json", "path to the immediate next hop's canonical JSON findings (always written, regardless of --output)")
	cmd.Flags().StringVar(&provider, "provider", "", "cloud provider for enrichment checks: eks, aks, gke, or omit for a cluster-only plan (aks/gke are recognized but not implemented yet — see docs/provider-roadmap.md)")
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
	cmd.Flags().StringVar(&listenAddress, "listen", "127.0.0.1:8080", "local report server listen address (falls back to a random free port if this one is busy, unless explicitly set)")
	cmd.Flags().StringVar(&terminalOutput, "terminal-output", "full", "stdout detail level: compact, full, or silent (default becomes compact when the local report server starts, unless set explicitly)")

	return cmd
}

// resolveFromVersion implements --from-version=auto's discovery order: an
// explicit value always wins; otherwise try EKS DescribeCluster
// (--provider=eks), then the cluster's own server version, and only fail
// if neither is available. Mirrors scan.go's "AWS enrichment is opt-in and
// must never hard-fail" pattern — a failed EKS lookup falls back instead
// of aborting.
func resolveFromVersion(ctx context.Context, explicit, provider, clusterName string, k8sCollector *k8s.Collector, out io.Writer, terminalMode string) (version, source string, err error) {
	if explicit != "" && explicit != "auto" {
		return explicit, "explicit-flag", nil
	}

	if provider == "eks" {
		if awsCollector, loadErr := awscol.LoadCollector(ctx, clusterName); loadErr != nil {
			if terminalMode != "silent" {
				fmt.Fprintf(out, "Could not auto-detect the current version via AWS (%v) — falling back to the cluster's own reported version.\n", loadErr)
			}
		} else if v, describeErr := awsCollector.DescribeClusterVersion(ctx); describeErr == nil {
			return v, "eks-describe-cluster", nil
		} else if terminalMode != "silent" {
			fmt.Fprintf(out, "Could not auto-detect the current version via EKS DescribeCluster (%v) — falling back to the cluster's own reported version.\n", describeErr)
		}
	}

	if v, err := k8sCollector.ServerVersion(); err == nil {
		return v, "k8s-server-version", nil
	}

	return "", "", fmt.Errorf("could not auto-detect the cluster's current version; pass --from-version explicitly, e.g. --from-version 1.29")
}

// isManifestOnlyFinding reports whether every resource reference on f comes
// from the manifest plane. API-001 can merge a live-cluster occurrence and
// a manifest occurrence of the same conceptual object into one finding
// (see internal/rules/api001.go's cross-plane merge) — such a finding is
// tied to live-cluster state and must not be projected forward as a future
// hop's PREDICTED finding, only a purely-manifest-sourced finding may be.
func isManifestOnlyFinding(f findings.Finding) bool {
	for _, ref := range f.Resources {
		if ref.Plane != findings.PlaneManifest {
			return false
		}
	}
	return len(f.Resources) > 0
}

// assessHop builds the HopReport for one hop beyond the first: rule
// categories that can be honestly re-evaluated (per plan.PolicyFor) become
// PREDICTED findings; everything else becomes a CarryForwardNote instead
// of a fabricated finding.
func assessHop(ctx context.Context, hop plan.Hop, sc *rules.ScanContext, reportContext, provider string, awsCollector *awscol.Collector, recommendedCommand string) (plan.HopReport, error) {
	var predicted []findings.Finding
	var carryForward []plan.CarryForwardNote

	// Manifest-projectable rules re-run against the *original* snapshot —
	// a YAML file's apiVersion doesn't change based on how many hops occur.
	for ruleID, rule := range manifestProjectableRules {
		if plan.PolicyFor(ruleID) != plan.ProjectFromManifests {
			continue
		}
		fs, err := rule.Evaluate(sc, hop.To)
		if err != nil {
			return plan.HopReport{}, fmt.Errorf("re-evaluating %s for hop %s: %w", ruleID, hop.To, err)
		}
		var hasLiveComponent bool
		for _, f := range fs {
			if isManifestOnlyFinding(f) {
				predicted = append(predicted, f)
			} else {
				hasLiveComponent = true
			}
		}
		if hasLiveComponent {
			carryForward = append(carryForward, plan.CarryForwardNote{
				RuleID:             ruleID,
				Reason:             "some " + ruleID + " findings are tied to a live cluster object (or a cross-plane match with one), which may be remediated or replaced before this hop is reached — only the pure-manifest findings above are projected forward",
				RecommendedCommand: recommendedCommand,
			})
		}
	}

	// AWS-projectable rules need a fresh AWS call for this hop's target
	// version — AWS's own API is authoritative for whatever version it's
	// asked about, unlike a live-cluster-state assumption.
	var needsFreshAWS bool
	for ruleID := range awsProjectableRules {
		if plan.PolicyFor(ruleID) == plan.ProjectFromFreshAWSQuery {
			needsFreshAWS = true
			break
		}
	}
	if needsFreshAWS {
		if provider == "eks" && awsCollector != nil {
			freshAWSSnap, err := awsCollector.Collect(ctx, hop.To)
			if err != nil {
				return plan.HopReport{}, fmt.Errorf("re-collecting AWS state for hop %s: %w", hop.To, err)
			}
			scratchSC := &rules.ScanContext{K8s: sc.K8s, AWS: freshAWSSnap, Manifests: sc.Manifests}
			for ruleID, rule := range awsProjectableRules {
				if plan.PolicyFor(ruleID) != plan.ProjectFromFreshAWSQuery {
					continue
				}
				fs, err := rule.Evaluate(scratchSC, hop.To)
				if err != nil {
					return plan.HopReport{}, fmt.Errorf("re-evaluating %s for hop %s: %w", ruleID, hop.To, err)
				}
				predicted = append(predicted, fs...)
			}
		} else {
			for ruleID := range awsProjectableRules {
				if plan.PolicyFor(ruleID) != plan.ProjectFromFreshAWSQuery {
					continue
				}
				carryForward = append(carryForward, plan.CarryForwardNote{
					RuleID:             ruleID,
					Reason:             "AWS enrichment is not available for this hop (no --provider=eks, or AWS credentials were unavailable for hop 1) — rerun with --provider=eks once this hop is reached to check " + ruleID,
					RecommendedCommand: recommendedCommand,
				})
			}
		}
	}

	// Everything else is carry-forward-only: current live-cluster state
	// (node versions, PDB overlap, webhook health, ...) that will likely
	// change by the time the cluster actually reaches this hop.
	for _, ruleID := range rules.NewDefaultRegistry().RuleIDs() {
		if _, isManifestRule := manifestProjectableRules[ruleID]; isManifestRule {
			continue
		}
		if _, isAWSRule := awsProjectableRules[ruleID]; isAWSRule {
			continue
		}
		carryForward = append(carryForward, plan.CarryForwardNote{
			RuleID:             ruleID,
			Reason:             ruleID + " describes current live-cluster state that will likely change by the time this hop is reached — rerun the scan once this hop is actually reached.",
			RecommendedCommand: recommendedCommand,
		})
	}

	var predictedReport *findings.Report
	if len(predicted) > 0 {
		predictedReport = findings.NewReport(hop.To, reportContext, provider, time.Now().UTC(), predicted)
	}

	return plan.HopReport{
		Hop:          hop,
		Status:       plan.HopStatusPredicted,
		Report:       predictedReport,
		CarryForward: carryForward,
	}, nil
}

// buildRecommendedScanCommand reconstructs the `scan` invocation a user
// should run once a future hop is actually reached, using the same flags
// this `plan` invocation received.
func buildRecommendedScanCommand(targetVersion, provider, clusterName string, manifestDirs, helmCharts, namespaceAllowlist []string) string {
	parts := []string{"kubepreflight", "scan", "--target-version", targetVersion}
	if provider != "" {
		parts = append(parts, "--provider", provider)
	}
	if clusterName != "" {
		parts = append(parts, "--cluster-name", clusterName)
	}
	for _, dir := range manifestDirs {
		parts = append(parts, "--manifests", dir)
	}
	for _, chart := range helmCharts {
		parts = append(parts, "--helm-chart", chart)
	}
	if len(namespaceAllowlist) > 0 {
		parts = append(parts, "--namespace-allowlist", strings.Join(namespaceAllowlist, ","))
	}
	return strings.Join(parts, " ")
}

// writePlanReportFile mirrors writeReportFile's create/write/close-
// explicitly pattern for the plan-specific upgrade-plan.json artifact.
func writePlanReportFile(path string, p *plan.PlanReport) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	if err := report.WritePlanJSON(p, f); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}
	return nil
}
