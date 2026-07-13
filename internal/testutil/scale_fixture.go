package testutil

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"kubepreflight/internal/collectors/k8s"
)

// ScaleFixtureConfig describes a synthetic cluster inventory for scale
// benchmarks. AddOnWorkloadCount is a subset of DeploymentCount: those
// Deployments are named like well-known add-ons so existing live add-on
// rules can evaluate them without a separate fake model.
type ScaleFixtureConfig struct {
	Name                  string
	NamespaceCount        int
	PodCount              int
	DeploymentCount       int
	StatefulSetCount      int
	DaemonSetCount        int
	PDBCount              int
	CRDCount              int
	AdmissionWebhookCount int
	NodeCount             int
	AddOnWorkloadCount    int
	RiskyObjectCount      int
}

// ScaleFixture is the generated benchmark inventory. Namespaces are kept
// separately because the production Snapshot intentionally does not collect
// Namespace objects today, while scale tests still need to verify namespace
// composition.
type ScaleFixture struct {
	Config     ScaleFixtureConfig
	Namespaces []corev1.Namespace
	Snapshot   *k8s.Snapshot
}

// ScaleScenarioConfigs returns the canonical benchmark scenarios used by
// tests, benchmarks, and docs. Counts are deterministic and deliberately
// approximate real cluster sizes without committing large static fixtures.
func ScaleScenarioConfigs() []ScaleFixtureConfig {
	return []ScaleFixtureConfig{
		{
			Name: "small", NamespaceCount: 10, PodCount: 100, DeploymentCount: 20,
			StatefulSetCount: 4, DaemonSetCount: 3, PDBCount: 10, CRDCount: 10,
			AdmissionWebhookCount: 4, NodeCount: 10, AddOnWorkloadCount: 2, RiskyObjectCount: 4,
		},
		{
			Name: "medium", NamespaceCount: 100, PodCount: 1000, DeploymentCount: 200,
			StatefulSetCount: 40, DaemonSetCount: 20, PDBCount: 100, CRDCount: 100,
			AdmissionWebhookCount: 25, NodeCount: 100, AddOnWorkloadCount: 5, RiskyObjectCount: 40,
		},
		{
			Name: "large", NamespaceCount: 1000, PodCount: 10000, DeploymentCount: 800,
			StatefulSetCount: 120, DaemonSetCount: 60, PDBCount: 300, CRDCount: 300,
			AdmissionWebhookCount: 80, NodeCount: 500, AddOnWorkloadCount: 8, RiskyObjectCount: 120,
		},
	}
}

func ScaleScenarioConfig(name string) (ScaleFixtureConfig, bool) {
	for _, cfg := range ScaleScenarioConfigs() {
		if cfg.Name == name {
			return cfg, true
		}
	}
	return ScaleFixtureConfig{}, false
}

func GenerateScaleFixture(cfg ScaleFixtureConfig) (*ScaleFixture, error) {
	if err := validateScaleFixtureConfig(cfg); err != nil {
		return nil, err
	}
	f := &ScaleFixture{
		Config: cfg,
		Snapshot: &k8s.Snapshot{
			Errors: map[string]error{},
		},
	}

	for i := 0; i < cfg.NamespaceCount; i++ {
		f.Namespaces = append(f.Namespaces, corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: scaleNamespace(i), UID: scaleUID("namespace", i)},
		})
	}
	for i := 0; i < cfg.NodeCount; i++ {
		f.Snapshot.Nodes = append(f.Snapshot.Nodes, scaleNode(i))
	}
	for i := 0; i < cfg.DeploymentCount; i++ {
		f.Snapshot.Deployments = append(f.Snapshot.Deployments, scaleDeployment(cfg, i))
	}
	for i := 0; i < cfg.StatefulSetCount; i++ {
		f.Snapshot.StatefulSets = append(f.Snapshot.StatefulSets, scaleStatefulSet(cfg, i))
	}
	for i := 0; i < cfg.DaemonSetCount; i++ {
		f.Snapshot.DaemonSets = append(f.Snapshot.DaemonSets, scaleDaemonSet(cfg, i))
	}
	for i := 0; i < cfg.PodCount; i++ {
		f.Snapshot.Pods = append(f.Snapshot.Pods, scalePod(cfg, i))
	}
	for i := 0; i < cfg.PDBCount; i++ {
		f.Snapshot.PodDisruptionBudgets = append(f.Snapshot.PodDisruptionBudgets, scalePDB(cfg, i))
	}
	for i := 0; i < cfg.CRDCount; i++ {
		f.Snapshot.CustomResourceDefinitions = append(f.Snapshot.CustomResourceDefinitions, scaleCRD(cfg, i))
	}
	for i := 0; i < cfg.AdmissionWebhookCount; i++ {
		f.Snapshot.ValidatingWebhookConfigs = append(f.Snapshot.ValidatingWebhookConfigs, scaleWebhookConfig(cfg, i))
	}
	return f, nil
}

func validateScaleFixtureConfig(cfg ScaleFixtureConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("scale fixture name is required")
	}
	for label, value := range map[string]int{
		"NamespaceCount":        cfg.NamespaceCount,
		"PodCount":              cfg.PodCount,
		"DeploymentCount":       cfg.DeploymentCount,
		"StatefulSetCount":      cfg.StatefulSetCount,
		"DaemonSetCount":        cfg.DaemonSetCount,
		"PDBCount":              cfg.PDBCount,
		"CRDCount":              cfg.CRDCount,
		"AdmissionWebhookCount": cfg.AdmissionWebhookCount,
		"NodeCount":             cfg.NodeCount,
		"AddOnWorkloadCount":    cfg.AddOnWorkloadCount,
		"RiskyObjectCount":      cfg.RiskyObjectCount,
	} {
		if value < 0 {
			return fmt.Errorf("%s must be >= 0", label)
		}
	}
	if cfg.NamespaceCount == 0 {
		return fmt.Errorf("NamespaceCount must be > 0")
	}
	if cfg.NodeCount == 0 && cfg.PodCount > 0 {
		return fmt.Errorf("NodeCount must be > 0 when PodCount is > 0")
	}
	if cfg.AddOnWorkloadCount > cfg.DeploymentCount {
		return fmt.Errorf("AddOnWorkloadCount must be <= DeploymentCount")
	}
	return nil
}

func scaleNamespace(i int) string {
	return fmt.Sprintf("scale-ns-%04d", i)
}

func scaleUID(kind string, i int) types.UID {
	return types.UID(fmt.Sprintf("scale-%s-%06d", kind, i))
}

func scaleNode(i int) corev1.Node {
	labels := map[string]string{
		"kubernetes.io/hostname":      fmt.Sprintf("scale-node-%04d", i),
		"kubernetes.io/os":            "linux",
		"topology.kubernetes.io/zone": fmt.Sprintf("zone-%d", i%3),
	}
	if i == 0 {
		labels["scale-role"] = "scarce"
	}
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("scale-node-%04d", i), UID: scaleUID("node", i), Labels: labels},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.33.0"},
			Conditions: []corev1.NodeCondition{{
				Type: corev1.NodeReady, Status: corev1.ConditionTrue,
			}},
		},
	}
}

func scaleDeployment(cfg ScaleFixtureConfig, i int) appsv1.Deployment {
	namespace := scaleNamespace(i % cfg.NamespaceCount)
	name := scaleDeploymentName(cfg, i)
	replicas := int32(3)
	ready := int32(3)
	labels := map[string]string{"app": name}
	podSpec := scalePodSpec(name, "")
	if i < cfg.RiskyObjectCount {
		replicas = 1
		ready = 1
		podSpec.NodeSelector = map[string]string{"scale-role": "scarce"}
	}
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: scaleUID("deployment", i), Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: podSpec},
		},
		Status: appsv1.DeploymentStatus{Replicas: replicas, ReadyReplicas: ready, AvailableReplicas: ready},
	}
}

func scaleDeploymentName(cfg ScaleFixtureConfig, i int) string {
	addons := []string{
		"metrics-server",
		"ingress-nginx-controller",
		"cert-manager",
		"external-dns",
		"aws-load-balancer-controller",
	}
	if i < cfg.AddOnWorkloadCount {
		if i < len(addons) {
			return addons[i]
		}
		return fmt.Sprintf("external-dns-%02d", i)
	}
	return fmt.Sprintf("scale-deploy-%06d", i)
}

func scaleStatefulSet(cfg ScaleFixtureConfig, i int) appsv1.StatefulSet {
	namespace := scaleNamespace((i + cfg.DeploymentCount) % cfg.NamespaceCount)
	name := fmt.Sprintf("scale-sts-%06d", i)
	replicas := int32(3)
	ready := replicas
	if i < cfg.RiskyObjectCount/10+1 {
		ready = replicas - 1
	}
	labels := map[string]string{"app": name}
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: scaleUID("statefulset", i), Labels: labels},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: scalePodSpec(name, "")},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas: replicas, ReadyReplicas: ready, CurrentReplicas: ready, UpdatedReplicas: ready,
			CurrentRevision: "rev-1", UpdateRevision: "rev-1",
		},
	}
}

func scaleDaemonSet(cfg ScaleFixtureConfig, i int) appsv1.DaemonSet {
	namespace := scaleNamespace((i + cfg.DeploymentCount + cfg.StatefulSetCount) % cfg.NamespaceCount)
	name := fmt.Sprintf("scale-ds-%06d", i)
	desired := int32(cfg.NodeCount)
	ready := desired
	unavailable := int32(0)
	if i < cfg.RiskyObjectCount/20+1 && desired > 1 {
		ready = desired - 1
		unavailable = 1
	}
	labels := map[string]string{"app": name}
	return appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: scaleUID("daemonset", i), Labels: labels},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: scalePodSpec(name, "")},
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: desired,
			NumberReady:            ready,
			NumberAvailable:        ready,
			NumberUnavailable:      unavailable,
		},
	}
}

func scalePod(cfg ScaleFixtureConfig, i int) corev1.Pod {
	namespace := scaleNamespace(i % cfg.NamespaceCount)
	controllerIndex := i % max(1, cfg.DeploymentCount)
	app := scaleDeploymentName(cfg, controllerIndex)
	nodeName := fmt.Sprintf("scale-node-%04d", i%cfg.NodeCount)
	owner := metav1.OwnerReference{APIVersion: "apps/v1", Kind: "Deployment", Name: app, UID: scaleUID("deployment", controllerIndex)}
	if cfg.DaemonSetCount > 0 && i%97 == 0 {
		dsIndex := i % cfg.DaemonSetCount
		owner = metav1.OwnerReference{APIVersion: "apps/v1", Kind: "DaemonSet", Name: fmt.Sprintf("scale-ds-%06d", dsIndex), UID: scaleUID("daemonset", dsIndex)}
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       namespace,
			Name:            fmt.Sprintf("scale-pod-%06d", i),
			UID:             scaleUID("pod", i),
			Labels:          map[string]string{"app": app},
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		Spec:   scalePodSpec(app, nodeName),
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func scalePodSpec(app, nodeName string) corev1.PodSpec {
	return corev1.PodSpec{
		NodeName: nodeName,
		Containers: []corev1.Container{{
			Name:  app,
			Image: fmt.Sprintf("example.com/%s:v1.0.0", app),
			Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			}},
		}},
	}
}

func scalePDB(cfg ScaleFixtureConfig, i int) policyv1.PodDisruptionBudget {
	namespace := scaleNamespace(i % cfg.NamespaceCount)
	app := scaleDeploymentName(cfg, i%max(1, cfg.DeploymentCount))
	minAvailable := intstr.FromInt(2)
	disruptionsAllowed := int32(1)
	if i < cfg.RiskyObjectCount {
		disruptionsAllowed = 0
	}
	return policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: fmt.Sprintf("scale-pdb-%06d", i), UID: scaleUID("pdb", i), Generation: 1},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": app}},
		},
		Status: policyv1.PodDisruptionBudgetStatus{
			ObservedGeneration: 1,
			DisruptionsAllowed: disruptionsAllowed,
			CurrentHealthy:     2,
			DesiredHealthy:     2,
			ExpectedPods:       3,
		},
	}
}

func scaleCRD(cfg ScaleFixtureConfig, i int) apiextensionsv1.CustomResourceDefinition {
	group := fmt.Sprintf("scale-%06d.example.com", i)
	name := "widgets." + group
	versions := []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Served: true, Storage: true}}
	stored := []string{"v1"}
	if i < cfg.RiskyObjectCount/20+1 {
		versions = append([]apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1beta1", Served: true, Storage: false}}, versions...)
		stored = []string{"v1beta1", "v1"}
	}
	return apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: scaleUID("crd", i)},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: "widgets", Singular: "widget", Kind: "Widget", ListKind: "WidgetList",
			},
			Scope:    apiextensionsv1.NamespaceScoped,
			Versions: versions,
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: stored},
	}
}

func scaleWebhookConfig(cfg ScaleFixtureConfig, i int) admissionregistrationv1.ValidatingWebhookConfiguration {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore
	sideEffects := admissionregistrationv1.SideEffectClassNone
	timeout := int32(5)
	policy := &ignore
	rules := []admissionregistrationv1.RuleWithOperations{{
		Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
		Rule: admissionregistrationv1.Rule{
			APIGroups:   []string{"apps"},
			APIVersions: []string{"v1"},
			Resources:   []string{"deployments"},
		},
	}}
	clientConfig := admissionregistrationv1.WebhookClientConfig{
		URL:      ptr(fmt.Sprintf("https://scale-webhook-%06d.example.com/validate", i)),
		CABundle: []byte(scaleCABundlePEM),
	}
	if i < cfg.RiskyObjectCount/20+1 {
		policy = &fail
		rules = []admissionregistrationv1.RuleWithOperations{{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"*"},
				APIVersions: []string{"*"},
				Resources:   []string{"*"},
			},
		}}
		clientConfig = admissionregistrationv1.WebhookClientConfig{}
	}
	return admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("scale-webhook-%06d", i), UID: scaleUID("webhook", i)},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:                    fmt.Sprintf("scale-webhook-%06d.example.com", i),
			FailurePolicy:           policy,
			SideEffects:             &sideEffects,
			AdmissionReviewVersions: []string{"v1"},
			TimeoutSeconds:          &timeout,
			ClientConfig:            clientConfig,
			Rules:                   rules,
		}},
	}
}

func ptr[T any](v T) *T {
	return &v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// scaleCABundlePEM is a long-lived CA certificate used only to keep
// synthetic healthy webhook fixtures out of WH-004. The private key is not
// included and this certificate is not used for any network operation.
const scaleCABundlePEM = `-----BEGIN CERTIFICATE-----
MIIBjDCCATGgAwIBAgIUaULiw8uT53Uz+y/2shuJhkr+tlcwCgYIKoZIzj0EAwIw
GzEZMBcGA1UEAwwQc2NhbGUtZml4dHVyZS1jYTAeFw0yNjA3MTMwNDAxMzJaFw0z
NjA3MTAwNDAxMzJaMBsxGTAXBgNVBAMMEHNjYWxlLWZpeHR1cmUtY2EwWTATBgcq
hkjOPQIBBggqhkjOPQMBBwNCAARjJdOhhtKAbBKR8kzGkAlajHBK1YMNWr+yFHxG
rJHJPdXbT7oDTYBoKTFwVPsSNstTIykE3ibOsk6D8hCWzg+Ro1MwUTAdBgNVHQ4E
FgQUcyJIN4iMryZWTKdj0yQen9fY5zcwHwYDVR0jBBgwFoAUcyJIN4iMryZWTKdj
0yQen9fY5zcwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNJADBGAiEArQfd
He2GVRB2iHDE7UKV39/64Zew9L05NZ5KR5+iGx4CIQCox25W0C1GH3aEC7+fZm+0
vkblJ4p7pa0nb+ImZcxUDw==
-----END CERTIFICATE-----
`
