// Package manifest collects deprecated-API usage from static Kubernetes
// manifests: raw YAML directories and rendered Helm charts. This is Plane
// 1 from the deep dive's 3-plane model (Section 4.2) — it never touches a
// live cluster, only the local filesystem and the local `helm` binary.
package manifest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"kubepreflight/internal/apicatalog"
)

// DeprecatedAPIObject is one manifest-defined object at a deprecated API
// group/version, found via static parsing rather than a live cluster call.
// There is no UID (the object may never have been applied to a cluster);
// SourcePath is the stable identity instead.
type DeprecatedAPIObject struct {
	apicatalog.DeprecatedAPI
	Namespace  string
	Name       string
	SourcePath string // file path, or "helm:<chart path>" for rendered charts
}

// WorkloadObject is one manifest-defined workload kind with a structured
// pod spec path that rules can inspect without fuzzy text scanning.
type WorkloadObject struct {
	Kind        string
	Namespace   string
	Name        string
	SourcePath  string
	Labels      map[string]string
	PodSpec     map[string]interface{}
	PodSpecPath string
}

// Snapshot is the read-only static-manifest state a scan operates on.
type Snapshot struct {
	DeprecatedAPIUsage []DeprecatedAPIObject
	Workloads          []WorkloadObject

	// Errors records directories/charts that failed to scan/render, keyed
	// by source, so a scan can report partial manifest results instead of
	// dropping this plane entirely — same principle as the k8s and AWS
	// collectors.
	Errors map[string]error
}

// HelmChart is one chart to render via `helm template` before scanning.
type HelmChart struct {
	Path        string   // local chart directory
	ReleaseName string   // release name passed to `helm template`
	ValuesFiles []string // -f flags, applied in order
}

// Collector scans manifest directories and rendered Helm charts for
// objects at deprecated/removed API groups (internal/apicatalog).
type Collector struct {
	manifestDirs []string
	helmCharts   []HelmChart
}

// NewCollector builds a Collector over the given raw manifest directories
// and Helm charts. Either may be empty.
func NewCollector(manifestDirs []string, helmCharts []HelmChart) *Collector {
	return &Collector{manifestDirs: manifestDirs, helmCharts: helmCharts}
}

// Collect walks every manifest directory and renders every Helm chart,
// recording deprecated-API matches. A failure in one directory or chart is
// recorded in Snapshot.Errors and does not abort the others.
func (c *Collector) Collect(ctx context.Context) (*Snapshot, error) {
	snap := &Snapshot{Errors: map[string]error{}}

	for _, dir := range c.manifestDirs {
		if err := c.scanDir(dir, snap); err != nil {
			snap.Errors["manifest-dir:"+dir] = err
		}
	}

	for _, chart := range c.helmCharts {
		if err := c.scanHelmChart(ctx, chart, snap); err != nil {
			snap.Errors["helm-chart:"+chart.Path] = err
		}
	}

	return snap, nil
}

func (c *Collector) scanDir(dir string, snap *Snapshot) error {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Manifest path not found. Check the path or remove --manifests: %s", dir)
		}
		return fmt.Errorf("checking manifest path %s: %w", dir, err)
	}
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if err := matchManifestObjects(raw, relativeSourcePath(dir, path), snap, true); err != nil {
			snap.Errors["manifest-file:"+path] = err
		}
		return nil
	})
}

// relativeSourcePath turns a manifest's on-disk path into a display-safe
// form: relative to the scanned --manifests root whenever the two differ
// (a directory scan), or just the file's own basename when root and path
// are the same file (root itself points directly at a single file). This
// keeps enough information to locate the manifest within the scanned root
// without embedding the operator's absolute local path — username, repo
// name, directory layout — into findings.json/report.html/report.md/the
// Console, which get shared outside the machine that ran the scan far
// more often than the raw filesystem layout is actually needed there.
func relativeSourcePath(root, path string) string {
	if root == path {
		return filepath.Base(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(path)
	}
	return filepath.ToSlash(rel)
}

func (c *Collector) scanHelmChart(ctx context.Context, chart HelmChart, snap *Snapshot) error {
	releaseName := chart.ReleaseName
	if releaseName == "" {
		releaseName = filepath.Base(chart.Path)
	}

	args := []string{"template", releaseName, chart.Path}
	for _, vf := range chart.ValuesFiles {
		args = append(args, "-f", vf)
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm template %s: %w (stderr: %s)", chart.Path, err, strings.TrimSpace(stderr.String()))
	}

	return matchManifestObjects(stdout.Bytes(), "helm:"+chart.Path, snap, false)
}

// matchDeprecatedAPIs decodes a multi-document YAML/JSON stream and
// records every document whose apiVersion/kind matches an
// apicatalog.Deprecated entry. Individual documents that fail to decode
// (Helm output commonly includes "# Source:" comment-only chunks and
// empty documents between "---" separators) are skipped rather than
// aborting the whole file/chart.
func matchDeprecatedAPIs(raw []byte, sourcePath string, snap *Snapshot) error {
	return matchManifestObjects(raw, sourcePath, snap, false)
}

// matchManifestObjects decodes a multi-document YAML/JSON stream and
// records every supported structured object. Helm-rendered streams reuse
// this for deprecated API detection, but intentionally do not populate
// Workloads yet; NODE-003 manifest workload scanning is scoped to raw
// --manifests inputs in this PR.
func matchManifestObjects(raw []byte, sourcePath string, snap *Snapshot, includeWorkloads bool) error {
	var firstErr error
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 4096)
	for {
		var obj unstructured.Unstructured
		err := decoder.Decode(&obj.Object)
		if err == io.EOF {
			return firstErr
		}
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("decoding %s: %w", sourcePath, err)
			}
			continue
		}
		if obj.Object == nil {
			continue
		}

		gvk := obj.GroupVersionKind()
		if gvk.Kind == "" {
			continue // not a Kubernetes object document
		}

		for _, dep := range apicatalog.Deprecated {
			if dep.Group == gvk.Group && dep.Version == gvk.Version && dep.Kind == gvk.Kind {
				snap.DeprecatedAPIUsage = append(snap.DeprecatedAPIUsage, DeprecatedAPIObject{
					DeprecatedAPI: dep,
					Namespace:     obj.GetNamespace(),
					Name:          obj.GetName(),
					SourcePath:    sourcePath,
				})
				break
			}
		}
		if includeWorkloads {
			if workload, ok := manifestWorkloadObject(obj, sourcePath); ok {
				snap.Workloads = append(snap.Workloads, workload)
			}
		}
	}
}

func manifestWorkloadObject(obj unstructured.Unstructured, sourcePath string) (WorkloadObject, bool) {
	podSpecPath, fields := manifestPodSpecPath(obj)
	if podSpecPath == "" {
		return WorkloadObject{}, false
	}
	podSpec, ok, _ := unstructured.NestedMap(obj.Object, fields...)
	if !ok || podSpec == nil {
		return WorkloadObject{}, false
	}
	return WorkloadObject{
		Kind:        obj.GetKind(),
		Namespace:   obj.GetNamespace(),
		Name:        obj.GetName(),
		SourcePath:  sourcePath,
		Labels:      obj.GetLabels(),
		PodSpec:     podSpec,
		PodSpecPath: podSpecPath,
	}, true
}

func manifestPodSpecPath(obj unstructured.Unstructured) (string, []string) {
	switch obj.GetKind() {
	case "Deployment", "DaemonSet", "StatefulSet", "Job":
		return "spec.template.spec", []string{"spec", "template", "spec"}
	case "CronJob":
		return "spec.jobTemplate.spec.template.spec", []string{"spec", "jobTemplate", "spec", "template", "spec"}
	case "Pod":
		return "spec", []string{"spec"}
	default:
		return "", nil
	}
}
