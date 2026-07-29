package rules

import (
	"strings"
)

// EvidencePlane identifies the collector plane a rule dependency reads.
type EvidencePlane string

const (
	EvidencePlaneKubernetes EvidencePlane = "kubernetes"
	EvidencePlaneAWS        EvidencePlane = "aws"
	EvidencePlaneManifests  EvidencePlane = "manifests"
)

// EvidenceDependency is a rule-owned statement that a missing collector
// result makes the rule's "no finding" result non-authoritative.
//
// Prefix marks Key as a prefix match for dynamic collector keys such as
// deprecated-api:<gvr> or describe-addon:<name>. Optional dependencies are
// deliberately not represented here yet; every dependency in this list is a
// core condition for the rule to evaluate truthfully.
type EvidenceDependency struct {
	Plane  EvidencePlane
	Key    string
	Prefix bool
}

func (d EvidenceDependency) Label() string {
	if d.Key == "" {
		return string(d.Plane)
	}
	if d.Prefix {
		return string(d.Plane) + ":" + d.Key + "*"
	}
	return string(d.Plane) + ":" + d.Key
}

func k8sDep(key string) EvidenceDependency {
	return EvidenceDependency{Plane: EvidencePlaneKubernetes, Key: key}
}

func k8sPrefixDep(key string) EvidenceDependency {
	return EvidenceDependency{Plane: EvidencePlaneKubernetes, Key: key, Prefix: true}
}

func awsDep(key string) EvidenceDependency {
	return EvidenceDependency{Plane: EvidencePlaneAWS, Key: key}
}

func awsPrefixDep(key string) EvidenceDependency {
	return EvidenceDependency{Plane: EvidencePlaneAWS, Key: key, Prefix: true}
}

func manifestDep() EvidenceDependency {
	return EvidenceDependency{Plane: EvidencePlaneManifests}
}

type dependencyStateValue int

const (
	dependencyNotApplicable dependencyStateValue = iota
	dependencyAvailable
	dependencyPartial
	dependencyMissing
)

func dependencyState(dep EvidenceDependency, sc *ScanContext) dependencyStateValue {
	if sc == nil {
		return dependencyMissing
	}
	switch dep.Plane {
	case EvidencePlaneKubernetes:
		requested := sc.KubernetesRequested || sc.K8s != nil
		if !requested {
			return dependencyNotApplicable
		}
		if sc.K8s == nil {
			return dependencyMissing
		}
		if dependencyErrored(dep, sc.K8s.Errors) {
			return dependencyPartial
		}
		return dependencyAvailable
	case EvidencePlaneAWS:
		requested := sc.AWSRequested || sc.AWS != nil
		if !requested {
			return dependencyNotApplicable
		}
		if sc.AWS == nil {
			return dependencyMissing
		}
		if dependencyErrored(dep, sc.AWS.Errors) {
			return dependencyPartial
		}
		return dependencyAvailable
	case EvidencePlaneManifests:
		requested := sc.ManifestsRequested || sc.Manifests != nil
		if !requested {
			return dependencyNotApplicable
		}
		if sc.Manifests == nil {
			return dependencyMissing
		}
		if len(sc.Manifests.Errors) > 0 {
			return dependencyPartial
		}
		return dependencyAvailable
	default:
		return dependencyMissing
	}
}

func dependencyErrored(dep EvidenceDependency, errs map[string]error) bool {
	if len(errs) == 0 {
		return false
	}
	if dep.Key == "" {
		return true
	}
	if dep.Prefix {
		for key := range errs {
			if strings.HasPrefix(key, dep.Key) {
				return true
			}
		}
		return false
	}
	_, ok := errs[dep.Key]
	return ok
}

var (
	depDeprecatedAPIUsage = k8sPrefixDep("deprecated-api:")
	depNodes              = k8sDep("nodes")
	depPods               = k8sDep("pods")
	depPDBs               = k8sDep("poddisruptionbudgets")
	depValidatingWebhooks = k8sDep("validatingwebhookconfigurations")
	depMutatingWebhooks   = k8sDep("mutatingwebhookconfigurations")
	depServices           = k8sDep("services")
	depEndpointSlices     = k8sDep("endpointslices")
	depCRDs               = k8sDep("customresourcedefinitions")
	depDeployments        = k8sDep("deployments")
	depDaemonSets         = k8sDep("daemonsets")
	depStatefulSets       = k8sDep("statefulsets")
	depPVs                = k8sDep("persistentvolumes")
	depPVCs               = k8sDep("persistentvolumeclaims")
	depCoreDNS            = k8sDep("coredns-configmap")
	depAPIServices        = k8sDep("apiservices")

	depAWSDescribeCluster   = awsDep("describe-cluster")
	depAWSListInsights      = awsDep("list-insights")
	depAWSDescribeInsights  = awsPrefixDep("describe-insight:")
	depAWSListAddons        = awsDep("list-addons")
	depAWSDescribeAddons    = awsPrefixDep("describe-addon:")
	depAWSAddonVersions     = awsPrefixDep("describe-addon-versions:")
	depAWSListNodegroups    = awsDep("list-nodegroups")
	depAWSDescribeNodegroup = awsPrefixDep("describe-nodegroup:")
	depAWSSubnets           = awsDep("describe-subnets")
	depAWSSecurityGroups    = awsPrefixDep("describe-security-group:")
	depAWSVPCs              = awsPrefixDep("describe-vpc:")
)

func (API001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depDeprecatedAPIUsage, manifestDep()}
}

func (API002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depDeprecatedAPIUsage, manifestDep()}
}

func (WH001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depValidatingWebhooks, depMutatingWebhooks}
}

func (WH002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depValidatingWebhooks, depMutatingWebhooks, depServices, depEndpointSlices}
}

func (WH002) EvidenceDependenciesFor(sc *ScanContext) []EvidenceDependency {
	deps := []EvidenceDependency{depValidatingWebhooks, depMutatingWebhooks}
	if sc == nil || sc.K8s == nil {
		return deps
	}
	if dependencyErrored(depValidatingWebhooks, sc.K8s.Errors) || dependencyErrored(depMutatingWebhooks, sc.K8s.Errors) {
		return deps
	}
	if !webhookServiceReferencePresent(sc) {
		return deps
	}
	deps = append(deps, depServices)
	if !dependencyErrored(depServices, sc.K8s.Errors) && webhookReferencedServiceObserved(sc) {
		deps = append(deps, depEndpointSlices)
	}
	return deps
}

func (WH004) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depValidatingWebhooks, depMutatingWebhooks}
}

func (WH005) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depValidatingWebhooks, depMutatingWebhooks}
}

func (DRAIN001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depDeployments, depStatefulSets, depPods, depPDBs}
}

func (DRAIN001) EvidenceDependenciesFor(sc *ScanContext) []EvidenceDependency {
	deps := []EvidenceDependency{depDeployments, depStatefulSets}
	if sc == nil || sc.K8s == nil {
		return deps
	}
	if dependencyErrored(depDeployments, sc.K8s.Errors) || dependencyErrored(depStatefulSets, sc.K8s.Errors) {
		return deps
	}
	for _, d := range sc.K8s.Deployments {
		if d.DeletionTimestamp == nil && isSingletonReplicaCount(d.Spec.Replicas) {
			return append(deps, depPods, depPDBs)
		}
	}
	for _, sts := range sc.K8s.StatefulSets {
		if sts.DeletionTimestamp == nil && isSingletonReplicaCount(sts.Spec.Replicas) {
			return append(deps, depPods, depPDBs)
		}
	}
	return deps
}

func (DRAIN002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depDeployments, depStatefulSets, depPods, depNodes, depPVs, depPVCs}
}

func (DRAIN003) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depDeployments, depStatefulSets, depDaemonSets, depPods, depNodes}
}

func (DRAIN004) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depNodes, depPods}
}

func (DRAIN005) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depStatefulSets, depDaemonSets}
}

func (PDB001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depPDBs}
}

func (PDB002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depPDBs, depPods}
}

func (NODE001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depNodes}
}

func (NODE002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSDescribeCluster, depAWSSubnets}
}

func (NODE003) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depDeployments, depDaemonSets, depStatefulSets}
}

func (NET002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSDescribeCluster, depAWSSecurityGroups, depAWSVPCs}
}

func (WORKLOAD001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depPods}
}

func (ADDON001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListAddons, depAWSDescribeAddons, depAWSAddonVersions, depDeployments, depDaemonSets}
}

func (ADDON002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListAddons, depAWSDescribeAddons, depAWSAddonVersions, depDeployments, depDaemonSets}
}

func (EKSNG001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListNodegroups, depAWSDescribeNodegroup}
}

func (EKSNG002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListNodegroups, depAWSDescribeNodegroup}
}

func (EKSNG003) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListNodegroups, depAWSDescribeNodegroup}
}

func (EKSNG004) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListNodegroups, depAWSDescribeNodegroup}
}

func (EKSINSIGHT001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListInsights, depAWSDescribeInsights}
}

func (EKSINSIGHT002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListInsights, depAWSDescribeInsights}
}

func (EKSINSIGHT003) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAWSListInsights, depAWSDescribeInsights}
}

func (COREDNS001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depCoreDNS}
}

func (CRD001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depCRDs}
}

func (CRD002) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depCRDs, depServices, depEndpointSlices}
}

func (CRD002) EvidenceDependenciesFor(sc *ScanContext) []EvidenceDependency {
	deps := []EvidenceDependency{depCRDs}
	if sc == nil || sc.K8s == nil || dependencyErrored(depCRDs, sc.K8s.Errors) {
		return deps
	}
	if !crdConversionServiceReferencePresent(sc) {
		return deps
	}
	deps = append(deps, depServices)
	if !dependencyErrored(depServices, sc.K8s.Errors) && crdConversionServiceObserved(sc) {
		deps = append(deps, depEndpointSlices)
	}
	return deps
}

func (APIService001) EvidenceDependencies() []EvidenceDependency {
	return []EvidenceDependency{depAPIServices}
}

func webhookServiceReferencePresent(sc *ScanContext) bool {
	for _, cfg := range sc.K8s.ValidatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			if wh.ClientConfig.Service != nil {
				return true
			}
		}
	}
	for _, cfg := range sc.K8s.MutatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			if wh.ClientConfig.Service != nil {
				return true
			}
		}
	}
	return false
}

func webhookReferencedServiceObserved(sc *ScanContext) bool {
	for _, cfg := range sc.K8s.ValidatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			if wh.ClientConfig.Service != nil && serviceExists(sc.K8s, wh.ClientConfig.Service.Namespace, wh.ClientConfig.Service.Name) {
				return true
			}
		}
	}
	for _, cfg := range sc.K8s.MutatingWebhookConfigs {
		for _, wh := range cfg.Webhooks {
			if wh.ClientConfig.Service != nil && serviceExists(sc.K8s, wh.ClientConfig.Service.Namespace, wh.ClientConfig.Service.Name) {
				return true
			}
		}
	}
	return false
}

func crdConversionServiceReferencePresent(sc *ScanContext) bool {
	for _, crd := range sc.K8s.CustomResourceDefinitions {
		conversion := crd.Spec.Conversion
		if conversion == nil || conversion.Webhook == nil || conversion.Webhook.ClientConfig == nil {
			continue
		}
		if conversion.Webhook.ClientConfig.Service != nil {
			return true
		}
	}
	return false
}

func crdConversionServiceObserved(sc *ScanContext) bool {
	for _, crd := range sc.K8s.CustomResourceDefinitions {
		conversion := crd.Spec.Conversion
		if conversion == nil || conversion.Webhook == nil || conversion.Webhook.ClientConfig == nil || conversion.Webhook.ClientConfig.Service == nil {
			continue
		}
		svc := conversion.Webhook.ClientConfig.Service
		if serviceExists(sc.K8s, svc.Namespace, svc.Name) {
			return true
		}
	}
	return false
}
