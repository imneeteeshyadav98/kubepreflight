// Package testutil provides fixture-loading helpers shared by tests across
// packages. Not built for production use.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/imneeteeshyadav98/kubepreflight/internal/apicatalog"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
)

var scheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := apiextensionsv1.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}()

// LoadFixtures reads every *.yaml file in dir, splits multi-document files
// on "---" separators, and decodes each document into a typed API object
// using the combined client-go + apiextensions scheme.
func LoadFixtures(dir string) ([]runtime.Object, error) {
	codecs := serializer.NewCodecFactory(scheme)
	decoder := codecs.UniversalDeserializer()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading fixtures dir %s: %w", dir, err)
	}

	var objs []runtime.Object
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		for _, doc := range strings.Split(string(raw), "\n---\n") {
			doc = strings.TrimSpace(doc)
			if doc == "" {
				continue
			}
			obj, _, err := decoder.Decode([]byte(doc), nil, nil)
			if err != nil {
				return nil, fmt.Errorf("decoding document in %s: %w", path, err)
			}
			objs = append(objs, obj)
		}
	}
	return objs, nil
}

// LoadUnstructuredFixtures reads every *.yaml file in dir, splits multi-doc
// files on "---", and decodes each document into an unstructured.Unstructured
// without requiring the type to be registered in any Go scheme. Used for
// fixtures representing removed API kinds (e.g. PodSecurityPolicy) that no
// longer have a corresponding Go struct in this project's client-go version.
func LoadUnstructuredFixtures(dir string) ([]runtime.Object, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading fixtures dir %s: %w", dir, err)
	}

	var objs []runtime.Object
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		for _, doc := range strings.Split(string(raw), "\n---\n") {
			doc = strings.TrimSpace(doc)
			if doc == "" {
				continue
			}
			var m map[string]interface{}
			if err := sigsyaml.Unmarshal([]byte(doc), &m); err != nil {
				return nil, fmt.Errorf("decoding document in %s: %w", path, err)
			}
			objs = append(objs, &unstructured.Unstructured{Object: m})
		}
	}
	return objs, nil
}

// NewFakeDynamicClient builds a fake dynamic client that knows how to list
// every GVR in apicatalog.Deprecated, seeded with the given objects
// (typically unstructured.Unstructured from LoadUnstructuredFixtures).
// Passing no objects gives a client that returns empty lists for every
// deprecated-API GVR — sufficient for tests that don't care about API-001.
func NewFakeDynamicClient(objs ...runtime.Object) dynamic.Interface {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, apicatalog.GVRToListKind(), objs...)
}

// SplitCRDs separates apiextensions CustomResourceDefinitions (which belong
// on the apiextensions fake clientset) from everything else (which belongs
// on the core kubernetes fake clientset).
func SplitCRDs(objs []runtime.Object) (core, crds []runtime.Object) {
	for _, o := range objs {
		if _, ok := o.(*apiextensionsv1.CustomResourceDefinition); ok {
			crds = append(crds, o)
		} else {
			core = append(core, o)
		}
	}
	return core, crds
}

// BuildSnapshot assembles a k8s.Snapshot directly from decoded fixture
// objects, bypassing the clientset/collector round trip. Rule-level tests
// use this to go straight from fixtures to Rule.Evaluate.
func BuildSnapshot(objs []runtime.Object) *k8s.Snapshot {
	snap := &k8s.Snapshot{Errors: map[string]error{}}
	for _, o := range objs {
		switch v := o.(type) {
		case *corev1.Node:
			snap.Nodes = append(snap.Nodes, *v)
		case *corev1.Pod:
			snap.Pods = append(snap.Pods, *v)
		case *policyv1.PodDisruptionBudget:
			snap.PodDisruptionBudgets = append(snap.PodDisruptionBudgets, *v)
		case *admissionregistrationv1.ValidatingWebhookConfiguration:
			snap.ValidatingWebhookConfigs = append(snap.ValidatingWebhookConfigs, *v)
		case *admissionregistrationv1.MutatingWebhookConfiguration:
			snap.MutatingWebhookConfigs = append(snap.MutatingWebhookConfigs, *v)
		case *corev1.Service:
			snap.Services = append(snap.Services, *v)
		case *discoveryv1.EndpointSlice:
			snap.EndpointSlices = append(snap.EndpointSlices, *v)
		case *apiextensionsv1.CustomResourceDefinition:
			snap.CustomResourceDefinitions = append(snap.CustomResourceDefinitions, *v)
		case *appsv1.Deployment:
			snap.Deployments = append(snap.Deployments, *v)
		case *appsv1.DaemonSet:
			snap.DaemonSets = append(snap.DaemonSets, *v)
		case *appsv1.StatefulSet:
			snap.StatefulSets = append(snap.StatefulSets, *v)
		case *corev1.PersistentVolume:
			snap.PersistentVolumes = append(snap.PersistentVolumes, *v)
		case *corev1.PersistentVolumeClaim:
			snap.PersistentVolumeClaims = append(snap.PersistentVolumeClaims, *v)
		case *corev1.ConfigMap:
			if v.Namespace == "kube-system" && v.Name == "coredns" {
				snap.CoreDNSConfigMap = v
			}
		case *unstructured.Unstructured:
			gvk := v.GroupVersionKind()
			for _, dep := range apicatalog.Deprecated {
				if dep.Group == gvk.Group && dep.Version == gvk.Version && dep.Kind == gvk.Kind {
					snap.DeprecatedAPIUsage = append(snap.DeprecatedAPIUsage, k8s.DeprecatedAPIObject{
						DeprecatedAPI: dep,
						Namespace:     v.GetNamespace(),
						Name:          v.GetName(),
						UID:           string(v.GetUID()),
						AutoManaged:   k8s.IsAutoManagedObject(dep, *v),
					})
					break
				}
			}
		}
	}
	return snap
}
