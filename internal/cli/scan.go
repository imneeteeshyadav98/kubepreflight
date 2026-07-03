package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/report"
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

func newScanCmd(exitCode *int) *cobra.Command {
	var kubeconfigPath string
	var kubeContext string
	var targetVersion string
	var output string
	var findingsOut string
	var provider string
	var clusterName string

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
			if provider != "" && provider != "eks" {
				return fmt.Errorf("--provider %q is not supported (only \"eks\" is supported currently, or omit --provider for a cluster-only scan)", provider)
			}
			if provider == "eks" && clusterName == "" {
				return fmt.Errorf("--cluster-name is required when --provider=eks")
			}

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
					fmt.Fprintf(cmd.OutOrStdout(), "AWS enrichment skipped (%v) — continuing with cluster-only checks.\n", err)
				} else {
					awsSnap, err = awsCollector.Collect(cmd.Context(), targetVersion)
					if err != nil {
						return fmt.Errorf("collecting AWS state: %w", err)
					}
				}
			}

			sc := &rules.ScanContext{K8s: snap, AWS: awsSnap}

			registry := rules.NewDefaultRegistry()
			fs, err := registry.RunAll(sc, targetVersion)
			if err != nil {
				return fmt.Errorf("running rules: %w", err)
			}

			rpt := findings.NewReport(targetVersion, reportContext, provider, time.Now().UTC(), fs)
			*exitCode = rpt.ExitCode()

			fmt.Fprintf(cmd.OutOrStdout(),
				"Collected: %d nodes, %d pods, %d PDBs, %d webhooks, %d services, %d endpointslices, %d CRDs, %d deployments, %d daemonsets | AWS enrichment: %v | Findings: %d\n\n",
				len(snap.Nodes), len(snap.Pods), len(snap.PodDisruptionBudgets),
				len(snap.ValidatingWebhookConfigs)+len(snap.MutatingWebhookConfigs),
				len(snap.Services), len(snap.EndpointSlices), len(snap.CustomResourceDefinitions),
				len(snap.Deployments), len(snap.DaemonSets), awsSnap != nil, len(fs))
			if len(snap.Errors) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Partial cluster scan — collectors failed: %v\n", snap.Errors)
			}
			if awsSnap != nil && len(awsSnap.Errors) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Partial AWS scan — collectors failed: %v\n", awsSnap.Errors)
			}

			if err := report.WriteTerminal(rpt, cmd.OutOrStdout()); err != nil {
				return fmt.Errorf("rendering terminal report: %w", err)
			}

			writeJSON := output == "json" || output == "all"
			writeMD := output == "md" || output == "all"
			writeHTML := output == "html" || output == "all"

			var writtenFiles []string
			if writeJSON {
				if err := writeReportFile(findingsOut, rpt, report.WriteJSON); err != nil {
					return err
				}
				writtenFiles = append(writtenFiles, findingsOut)
			}
			if writeMD {
				if err := writeReportFile("report.md", rpt, report.WriteMarkdown); err != nil {
					return err
				}
				writtenFiles = append(writtenFiles, "report.md")
			}
			if writeHTML {
				if err := writeReportFile("report.html", rpt, report.WriteHTML); err != nil {
					return err
				}
				writtenFiles = append(writtenFiles, "report.html")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nReports written: %s\n", strings.Join(writtenFiles, " · "))
			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", defaultKubeconfigPath(), "path to kubeconfig")
	cmd.Flags().StringVar(&kubeContext, "context", "", "kubeconfig context to use")
	cmd.Flags().StringVar(&targetVersion, "target-version", "", "target Kubernetes version, e.g. 1.34 (required)")
	cmd.Flags().StringVar(&output, "output", "json", "output format: json, md, html, or all")
	cmd.Flags().StringVar(&findingsOut, "findings-out", "findings.json", "path to write the JSON findings report")
	cmd.Flags().StringVar(&provider, "provider", "", "cloud provider for enrichment checks: \"eks\" or omit for a cluster-only scan")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "EKS cluster name (required when --provider=eks)")

	return cmd
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
