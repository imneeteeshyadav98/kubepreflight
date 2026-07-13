// Package k8s collects a read-only snapshot of cluster state used by the
// rules engine. It depends only on kubernetes.Interface so production code
// can inject a real clientset and tests can inject a fake one.
package k8s

import (
	"context"
	"fmt"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"kubepreflight/internal/apicatalog"
)

// Snapshot is the read-only cluster state a scan operates on. All lists are
// exactly the Week 1 collector scope; more resource kinds are added as later
// checks require them.
type Snapshot struct {
	Nodes                     []corev1.Node
	Pods                      []corev1.Pod
	PodDisruptionBudgets      []policyv1.PodDisruptionBudget
	ValidatingWebhookConfigs  []admissionregistrationv1.ValidatingWebhookConfiguration
	MutatingWebhookConfigs    []admissionregistrationv1.MutatingWebhookConfiguration
	Services                  []corev1.Service
	EndpointSlices            []discoveryv1.EndpointSlice
	CustomResourceDefinitions []apiextensionsv1.CustomResourceDefinition
	Deployments               []appsv1.Deployment
	DaemonSets                []appsv1.DaemonSet
	StatefulSets              []appsv1.StatefulSet
	PersistentVolumes         []corev1.PersistentVolume
	PersistentVolumeClaims    []corev1.PersistentVolumeClaim

	// DeprecatedAPIUsage holds live objects found at a group/version/resource
	// from apicatalog.Deprecated. Populated via the dynamic client, since the
	// removed API kinds it covers often no longer have a Go type in
	// k8s.io/api at all — there's no typed client to list them with.
	DeprecatedAPIUsage     []DeprecatedAPIObject
	UnavailableAPIServices []APIServiceAvailability

	// CoreDNSConfigMap is a single allowlisted Get of kube-system/coredns —
	// not a blanket ConfigMap list, per the "ConfigMap reads are allowlisted
	// to known add-on configs" security principle (deep dive Section 14.3).
	// Nil if the cluster has no CoreDNS ConfigMap by that name (e.g. a
	// different DNS provider) — this is not treated as a collection error.
	CoreDNSConfigMap *corev1.ConfigMap

	// Errors records collectors that failed, keyed by resource kind, so a
	// scan can report partial results instead of failing outright.
	Errors map[string]error
}

// DeprecatedAPIObject is one live object found at a deprecated/removed API
// group/version/resource.
type DeprecatedAPIObject struct {
	apicatalog.DeprecatedAPI
	Namespace string
	Name      string
	UID       string

	// AutoManaged reports whether a controller — not a person — owns and
	// continuously reconciles this object, so a removed-API hit on it
	// isn't a migration task the reader has to do by hand. Two distinct
	// signals feed this, both confirmed against a real cluster rather than
	// assumed:
	//   - flowcontrol.apiserver.k8s.io FlowSchema/PriorityLevelConfiguration:
	//     kube-apiserver's own bootstrap defaults carry the well-known
	//     apf.kubernetes.io/autoupdate-spec: "true" annotation. A
	//     user-created FlowSchema/PriorityLevelConfiguration never does.
	//   - discovery.k8s.io EndpointSlice: the built-in EndpointSlice
	//     controller labels every slice it creates with
	//     endpointslice.kubernetes.io/managed-by: endpointslice-controller.k8s.io
	//     (and sets a controller=true ownerReference to the owning
	//     Service). Both signals are absent on the one real exception
	//     observed — the default/kubernetes Service's own EndpointSlice,
	//     which some clusters create without going through that
	//     controller — so that one narrow case is conservatively left
	//     AutoManaged=false (a real Blocker) rather than guessed at.
	// Matching on either object kind's well-known label/annotation is
	// reliable and version-independent, unlike matching on default
	// object names, which would silently miss anything a future
	// Kubernetes release adds or renames.
	AutoManaged bool
}

// autoUpdateSpecAnnotation is the annotation kube-apiserver sets on its own
// bootstrap flowcontrol.apiserver.k8s.io defaults (FlowSchema and
// PriorityLevelConfiguration) to mark them as continuously reconciled.
const autoUpdateSpecAnnotation = "apf.kubernetes.io/autoupdate-spec"

// endpointSliceManagedByLabel is the label the built-in EndpointSlice
// controller sets on every EndpointSlice it creates — present regardless
// of Kubernetes version, absent on anything not written by that
// controller.
const endpointSliceManagedByLabel = "endpointslice.kubernetes.io/managed-by"

// IsAutoManagedObject reports whether item is one of the controller-owned
// object kinds DeprecatedAPIObject.AutoManaged documents — dispatches on
// Group/Kind since the two cases use different signals (an annotation for
// flowcontrol defaults, a label for EndpointSlice). Exported so
// internal/testutil's BuildSnapshot (used by fixture-driven rule tests)
// can share the exact same logic instead of a second, driftable copy.
func IsAutoManagedObject(dep apicatalog.DeprecatedAPI, item unstructured.Unstructured) bool {
	switch {
	case dep.Group == "flowcontrol.apiserver.k8s.io":
		return item.GetAnnotations()[autoUpdateSpecAnnotation] == "true"
	case dep.Group == "discovery.k8s.io" && dep.Kind == "EndpointSlice":
		return item.GetLabels()[endpointSliceManagedByLabel] != ""
	default:
		return false
	}
}

type APIServiceAvailability struct {
	Name, UID, Reason, Message string
}

// Collector gathers a Snapshot via the Kubernetes API. It never performs
// write operations.
type Collector struct {
	client        kubernetes.Interface
	apiExtCli     apiextensionsclientset.Interface
	dynamicClient dynamic.Interface
}

// NewCollector builds a Collector from already-constructed clients. Real
// callers pass clients built from kubeconfig; tests pass fake ones.
func NewCollector(client kubernetes.Interface, apiExtCli apiextensionsclientset.Interface, dynamicClient dynamic.Interface) *Collector {
	return &Collector{client: client, apiExtCli: apiExtCli, dynamicClient: dynamicClient}
}

// DefaultCollectorTimeout is the per-call budget Collect uses when the CLI
// doesn't override it via --collector-timeout. 30s comfortably covers a
// single List/Get against a healthy but large or momentarily slow cluster
// while still bounding a hung API server. This is a PER-CALL budget, not a
// budget for the whole Collect() -- see Collect's doc comment for what that
// means for total worst-case scan time against a fully unreachable server.
const DefaultCollectorTimeout = 30 * time.Second

// Collect lists every Week 1 resource kind cluster-wide. Failures on
// individual lists are recorded in Snapshot.Errors rather than aborting the
// whole collection, per the "never all-or-nothing" scan principle. Each
// List/Get gets its own timeout-bounded child of ctx (via collectResource),
// so one slow or hung call can time out and be recorded like any other
// collection failure without starving the calls that come after it. A
// timeout deadline exceeded needs no special-casing beyond that: it's
// recorded under the resource's own Snapshot.Errors key exactly like a
// permissions error would be, which is what already flips that plane's
// ScanCoverage to "partial" and the report's Result() to "INCOMPLETE" --
// see internal/cli/coverage.go and internal/findings/report.go. Cancelling
// ctx itself (e.g. via signal.NotifyContext for Ctrl+C, wired in
// internal/cli/scan.go) has the same effect: the in-flight call returns
// context.Canceled immediately rather than waiting out its own timeout.
//
// timeout is a PER-CALL budget: Collect makes roughly 50 sequential calls
// (a fixed dozen resource kinds plus one per entry in apicatalog.Deprecated),
// each getting its own fresh window. Against a completely unreachable API
// server, every single one uses its full budget before moving on, so total
// worst-case wall-clock time is on the order of (number of calls) *
// timeout -- confirmed against a real black-holed server address: ~50 calls
// at a 3s timeout took ~3 minutes end to end, correctly finishing with
// Result "INCOMPLETE" and exit code 3, never hanging indefinitely. This was
// a deliberate tradeoff over one shared budget for the whole method: a
// shared deadline bounds total time more tightly, but risks one slow early
// call (e.g. Nodes) starving every call after it even when they'd
// individually have been fast. Operators who want faster failure against a
// suspected-unreachable cluster should pass a smaller --collector-timeout
// rather than relying on this method to fail fast on its own.
func (c *Collector) Collect(ctx context.Context, timeout time.Duration) (*Snapshot, error) {
	snap := &Snapshot{Errors: map[string]error{}}

	c.collectResource(ctx, timeout, snap, "nodes", func(callCtx context.Context) error {
		v, err := c.client.CoreV1().Nodes().List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.Nodes = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "pods", func(callCtx context.Context) error {
		v, err := c.client.CoreV1().Pods(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.Pods = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "poddisruptionbudgets", func(callCtx context.Context) error {
		v, err := c.client.PolicyV1().PodDisruptionBudgets(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.PodDisruptionBudgets = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "validatingwebhookconfigurations", func(callCtx context.Context) error {
		v, err := c.client.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.ValidatingWebhookConfigs = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "mutatingwebhookconfigurations", func(callCtx context.Context) error {
		v, err := c.client.AdmissionregistrationV1().MutatingWebhookConfigurations().List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.MutatingWebhookConfigs = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "services", func(callCtx context.Context) error {
		v, err := c.client.CoreV1().Services(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.Services = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "endpointslices", func(callCtx context.Context) error {
		v, err := c.client.DiscoveryV1().EndpointSlices(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.EndpointSlices = v.Items
		return nil
	})

	if c.apiExtCli != nil {
		c.collectResource(ctx, timeout, snap, "customresourcedefinitions", func(callCtx context.Context) error {
			v, err := c.apiExtCli.ApiextensionsV1().CustomResourceDefinitions().List(callCtx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			snap.CustomResourceDefinitions = v.Items
			return nil
		})
	} else {
		snap.Errors["customresourcedefinitions"] = fmt.Errorf("apiextensions client not configured")
	}

	c.collectResource(ctx, timeout, snap, "deployments", func(callCtx context.Context) error {
		v, err := c.client.AppsV1().Deployments(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.Deployments = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "daemonsets", func(callCtx context.Context) error {
		v, err := c.client.AppsV1().DaemonSets(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.DaemonSets = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "statefulsets", func(callCtx context.Context) error {
		v, err := c.client.AppsV1().StatefulSets(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.StatefulSets = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "persistentvolumes", func(callCtx context.Context) error {
		v, err := c.client.CoreV1().PersistentVolumes().List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.PersistentVolumes = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "persistentvolumeclaims", func(callCtx context.Context) error {
		v, err := c.client.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		snap.PersistentVolumeClaims = v.Items
		return nil
	})

	c.collectResource(ctx, timeout, snap, "coredns-configmap", func(callCtx context.Context) error {
		cm, err := c.client.CoreV1().ConfigMaps("kube-system").Get(callCtx, "coredns", metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		snap.CoreDNSConfigMap = cm
		return nil
	})

	if c.dynamicClient != nil {
		for _, dep := range apicatalog.Deprecated {
			gvr := dep.GVR()
			c.collectResource(ctx, timeout, snap, "deprecated-api:"+gvr.String(), func(callCtx context.Context) error {
				list, err := c.dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(callCtx, metav1.ListOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						// The API server no longer serves this group/version
						// at all — expected on any cluster already past
						// removal, not a collection failure.
						return nil
					}
					return err
				}
				for _, item := range list.Items {
					snap.DeprecatedAPIUsage = append(snap.DeprecatedAPIUsage, DeprecatedAPIObject{
						DeprecatedAPI: dep,
						Namespace:     item.GetNamespace(),
						Name:          item.GetName(),
						UID:           string(item.GetUID()),
						AutoManaged:   IsAutoManagedObject(dep, item),
					})
				}
				return nil
			})
		}

		c.collectResource(ctx, timeout, snap, "apiservices", func(callCtx context.Context) error {
			apiServices, err := c.dynamicClient.Resource(schema.GroupVersionResource{Group: "apiregistration.k8s.io", Version: "v1", Resource: "apiservices"}).List(callCtx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, item := range apiServices.Items {
				conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
				available, reason, message := false, "", ""
				for _, raw := range conditions {
					condition, _ := raw.(map[string]any)
					if condition["type"] == "Available" {
						available = condition["status"] == "True"
						reason, _ = condition["reason"].(string)
						message, _ = condition["message"].(string)
					}
				}
				if !available {
					snap.UnavailableAPIServices = append(snap.UnavailableAPIServices, APIServiceAvailability{Name: item.GetName(), UID: string(item.GetUID()), Reason: reason, Message: message})
				}
			}
			return nil
		})
	} else {
		snap.Errors["deprecated-api-usage"] = fmt.Errorf("dynamic client not configured")
	}

	return snap, nil
}

// collectResource runs fn with a timeout-bounded child of ctx, recording any
// error it returns under key in snap.Errors. This is the single choke point
// every List/Get in Collect goes through, so a slow or hung API call times
// out on its own budget instead of blocking (or silently inheriting an
// unbounded wait from) the calls after it. fn returning nil after handling
// its own "not really an error" cases (e.g. apierrors.IsNotFound) suppresses
// recording entirely, same as the pre-timeout code's inline handling did.
func (c *Collector) collectResource(ctx context.Context, timeout time.Duration, snap *Snapshot, key string, fn func(context.Context) error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := fn(callCtx); err != nil {
		snap.Errors[key] = err
	}
}

// ServerVersion returns the cluster's current Kubernetes version (e.g.
// "v1.29.6-eks-1234567") via the discovery API. This is a separate, cheap,
// single call — not part of Collect's Snapshot — so callers that only need
// version discovery (the `plan` command's --from-version=auto path) don't
// pay for a full snapshot collection just to learn the version.
//
// client-go's own DiscoveryInterface.ServerVersion hardcodes context.TODO()
// internally (confirmed against client-go v0.31.3), so it can't be bounded
// by any caller-supplied context or timeout at all -- found by testing this
// package's timeout against a real black-holed API server address: scan/
// plan both call ServerVersion before Collect, so an unbounded version
// check ahead of an otherwise-bounded Collect would silently defeat the
// whole point of --collector-timeout. RESTClient()-based reimplementation
// was considered and rejected: DiscoveryInterface.RESTClient() returns nil
// on the fake discovery client client-go/discovery/fake ships (confirmed
// against v0.31.3), which would panic every existing test using
// fake.NewSimpleClientset() plus FakedServerVersion. Instead this runs the
// unbounded call in a goroutine and races it against timeout: the
// goroutine outlives a timed-out call (it keeps running until the
// underlying request actually resolves, since neither client-go nor Go's
// stdlib gives any other way to abort it), but the channel is buffered so
// it can always send its result and exit rather than leaking permanently
// once that happens -- a real network error/refusal returns almost
// immediately either way; only a genuinely black-holed address keeps it
// alive, and even then bounded by the OS's own TCP connect timeout (on the
// order of minutes, not truly forever).
func (c *Collector) ServerVersion(ctx context.Context, timeout time.Duration) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		version string
		err     error
	}
	resultCh := make(chan result, 1)
	go func() {
		info, err := c.client.Discovery().ServerVersion()
		if err != nil {
			resultCh <- result{err: fmt.Errorf("querying cluster server version: %w", err)}
			return
		}
		resultCh <- result{version: info.GitVersion}
	}()

	select {
	case r := <-resultCh:
		return r.version, r.err
	case <-callCtx.Done():
		return "", fmt.Errorf("querying cluster server version: %w", callCtx.Err())
	}
}
