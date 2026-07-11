// Package k8s collects a read-only snapshot of cluster state used by the
// rules engine. It depends only on kubernetes.Interface so production code
// can inject a real clientset and tests can inject a fake one.
package k8s

import (
	"context"
	"fmt"

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

	// AutoManaged reports whether kube-apiserver itself owns and
	// continuously reconciles this object — currently only meaningful for
	// flowcontrol.apiserver.k8s.io FlowSchema/PriorityLevelConfiguration,
	// which the API server marks with the well-known
	// apf.kubernetes.io/autoupdate-spec: "true" annotation on its own
	// bootstrap defaults (confirmed against a real cluster, not assumed).
	// A user-created FlowSchema/PriorityLevelConfiguration never carries
	// this annotation, so it's a reliable, version-independent signal —
	// unlike matching on the default objects' well-known names, which
	// would silently miss any name Kubernetes adds in a future release.
	AutoManaged bool
}

// autoUpdateSpecAnnotation is the annotation kube-apiserver sets on its own
// bootstrap flowcontrol.apiserver.k8s.io defaults (FlowSchema and
// PriorityLevelConfiguration) to mark them as continuously reconciled.
const autoUpdateSpecAnnotation = "apf.kubernetes.io/autoupdate-spec"

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

// Collect lists every Week 1 resource kind cluster-wide. Failures on
// individual lists are recorded in Snapshot.Errors rather than aborting the
// whole collection, per the "never all-or-nothing" scan principle.
func (c *Collector) Collect(ctx context.Context) (*Snapshot, error) {
	snap := &Snapshot{Errors: map[string]error{}}

	if v, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["nodes"] = err
	} else {
		snap.Nodes = v.Items
	}

	if v, err := c.client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["pods"] = err
	} else {
		snap.Pods = v.Items
	}

	if v, err := c.client.PolicyV1().PodDisruptionBudgets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["poddisruptionbudgets"] = err
	} else {
		snap.PodDisruptionBudgets = v.Items
	}

	if v, err := c.client.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["validatingwebhookconfigurations"] = err
	} else {
		snap.ValidatingWebhookConfigs = v.Items
	}

	if v, err := c.client.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["mutatingwebhookconfigurations"] = err
	} else {
		snap.MutatingWebhookConfigs = v.Items
	}

	if v, err := c.client.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["services"] = err
	} else {
		snap.Services = v.Items
	}

	if v, err := c.client.DiscoveryV1().EndpointSlices(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["endpointslices"] = err
	} else {
		snap.EndpointSlices = v.Items
	}

	if c.apiExtCli != nil {
		if v, err := c.apiExtCli.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{}); err != nil {
			snap.Errors["customresourcedefinitions"] = err
		} else {
			snap.CustomResourceDefinitions = v.Items
		}
	} else {
		snap.Errors["customresourcedefinitions"] = fmt.Errorf("apiextensions client not configured")
	}

	if v, err := c.client.AppsV1().Deployments(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["deployments"] = err
	} else {
		snap.Deployments = v.Items
	}

	if v, err := c.client.AppsV1().DaemonSets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{}); err != nil {
		snap.Errors["daemonsets"] = err
	} else {
		snap.DaemonSets = v.Items
	}

	if cm, err := c.client.CoreV1().ConfigMaps("kube-system").Get(ctx, "coredns", metav1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			snap.Errors["coredns-configmap"] = err
		}
	} else {
		snap.CoreDNSConfigMap = cm
	}

	if c.dynamicClient != nil {
		for _, dep := range apicatalog.Deprecated {
			gvr := dep.GVR()
			list, err := c.dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// The API server no longer serves this group/version at
					// all — expected on any cluster already past removal,
					// not a collection failure.
					continue
				}
				snap.Errors["deprecated-api:"+gvr.String()] = err
				continue
			}
			for _, item := range list.Items {
				snap.DeprecatedAPIUsage = append(snap.DeprecatedAPIUsage, DeprecatedAPIObject{
					DeprecatedAPI: dep,
					Namespace:     item.GetNamespace(),
					Name:          item.GetName(),
					UID:           string(item.GetUID()),
					AutoManaged:   item.GetAnnotations()[autoUpdateSpecAnnotation] == "true",
				})
			}
		}
		apiServices, err := c.dynamicClient.Resource(schema.GroupVersionResource{Group: "apiregistration.k8s.io", Version: "v1", Resource: "apiservices"}).List(ctx, metav1.ListOptions{})
		if err != nil {
			snap.Errors["apiservices"] = err
		} else {
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
		}
	} else {
		snap.Errors["deprecated-api-usage"] = fmt.Errorf("dynamic client not configured")
	}

	return snap, nil
}

// ServerVersion returns the cluster's current Kubernetes version (e.g.
// "v1.29.6-eks-1234567") via the discovery API. This is a separate, cheap,
// single call — not part of Collect's Snapshot — so callers that only need
// version discovery (the `plan` command's --from-version=auto path) don't
// pay for a full snapshot collection just to learn the version.
func (c *Collector) ServerVersion() (string, error) {
	info, err := c.client.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("querying cluster server version: %w", err)
	}
	return info.GitVersion, nil
}
