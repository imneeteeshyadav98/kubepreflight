# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | kp-cert-a |
| **Target version** | 1.33 |
| **Provider** | cluster-only |
| **Upgrade context** | audit_only |
| **Scanned at** | 2026-07-29 08:23:36 UTC |
| **Result** | **INCOMPLETE** |
| **Summary** | 0 blocker(s), 0 warning(s), 0 operator decision(s), 0 info(s) |

> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.

- Kubernetes: apiservices [permission-denied,permissionDenied,partialDataPreserved]: apiservices.apiregistration.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "apiservices" in API group "apiregistration.k8s.io" at the cluster scope
- Kubernetes: customresourcedefinitions [permission-denied,permissionDenied,partialDataPreserved]: customresourcedefinitions.apiextensions.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "customresourcedefinitions" in API group "apiextensions.k8s.io" at the cluster scope
- Kubernetes: daemonsets [permission-denied,permissionDenied,partialDataPreserved]: daemonsets.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "daemonsets" in API group "apps" at the cluster scope
- Kubernetes: deployments [permission-denied,permissionDenied,partialDataPreserved]: deployments.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "deployments" in API group "apps" at the cluster scope
- Kubernetes: deprecated-api:admissionregistration.k8s.io/v1beta1, Resource=mutatingwebhookconfigurations [permission-denied,permissionDenied,partialDataPreserved]: mutatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "mutatingwebhookconfigurations" in API group "admissionregistration.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:admissionregistration.k8s.io/v1beta1, Resource=validatingwebhookconfigurations [permission-denied,permissionDenied,partialDataPreserved]: validatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "validatingwebhookconfigurations" in API group "admissionregistration.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:apiextensions.k8s.io/v1beta1, Resource=customresourcedefinitions [permission-denied,permissionDenied,partialDataPreserved]: customresourcedefinitions.apiextensions.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "customresourcedefinitions" in API group "apiextensions.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:apiregistration.k8s.io/v1beta1, Resource=apiservices [permission-denied,permissionDenied,partialDataPreserved]: apiservices.apiregistration.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "apiservices" in API group "apiregistration.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:apps/v1beta1, Resource=deployments [permission-denied,permissionDenied,partialDataPreserved]: deployments.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "deployments" in API group "apps" at the cluster scope
- Kubernetes: deprecated-api:apps/v1beta1, Resource=statefulsets [permission-denied,permissionDenied,partialDataPreserved]: statefulsets.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "statefulsets" in API group "apps" at the cluster scope
- Kubernetes: deprecated-api:apps/v1beta2, Resource=daemonsets [permission-denied,permissionDenied,partialDataPreserved]: daemonsets.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "daemonsets" in API group "apps" at the cluster scope
- Kubernetes: deprecated-api:apps/v1beta2, Resource=deployments [permission-denied,permissionDenied,partialDataPreserved]: deployments.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "deployments" in API group "apps" at the cluster scope
- Kubernetes: deprecated-api:apps/v1beta2, Resource=replicasets [permission-denied,permissionDenied,partialDataPreserved]: replicasets.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "replicasets" in API group "apps" at the cluster scope
- Kubernetes: deprecated-api:apps/v1beta2, Resource=statefulsets [permission-denied,permissionDenied,partialDataPreserved]: statefulsets.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "statefulsets" in API group "apps" at the cluster scope
- Kubernetes: deprecated-api:autoscaling/v2beta1, Resource=horizontalpodautoscalers [permission-denied,permissionDenied,partialDataPreserved]: horizontalpodautoscalers.autoscaling is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "horizontalpodautoscalers" in API group "autoscaling" at the cluster scope
- Kubernetes: deprecated-api:autoscaling/v2beta2, Resource=horizontalpodautoscalers [permission-denied,permissionDenied,partialDataPreserved]: horizontalpodautoscalers.autoscaling is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "horizontalpodautoscalers" in API group "autoscaling" at the cluster scope
- Kubernetes: deprecated-api:batch/v1beta1, Resource=cronjobs [permission-denied,permissionDenied,partialDataPreserved]: cronjobs.batch is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "cronjobs" in API group "batch" at the cluster scope
- Kubernetes: deprecated-api:certificates.k8s.io/v1beta1, Resource=certificatesigningrequests [permission-denied,permissionDenied,partialDataPreserved]: certificatesigningrequests.certificates.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "certificatesigningrequests" in API group "certificates.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:coordination.k8s.io/v1beta1, Resource=leases [permission-denied,permissionDenied,partialDataPreserved]: leases.coordination.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "leases" in API group "coordination.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:discovery.k8s.io/v1beta1, Resource=endpointslices [permission-denied,permissionDenied,partialDataPreserved]: endpointslices.discovery.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "endpointslices" in API group "discovery.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:events.k8s.io/v1beta1, Resource=events [permission-denied,permissionDenied,partialDataPreserved]: events.events.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "events" in API group "events.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=daemonsets [permission-denied,permissionDenied,partialDataPreserved]: daemonsets.extensions is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "daemonsets" in API group "extensions" at the cluster scope
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=deployments [permission-denied,permissionDenied,partialDataPreserved]: deployments.extensions is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "deployments" in API group "extensions" at the cluster scope
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=ingresses [permission-denied,permissionDenied,partialDataPreserved]: ingresses.extensions is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "ingresses" in API group "extensions" at the cluster scope
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=networkpolicies [permission-denied,permissionDenied,partialDataPreserved]: networkpolicies.extensions is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "networkpolicies" in API group "extensions" at the cluster scope
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=podsecuritypolicies [permission-denied,permissionDenied,partialDataPreserved]: podsecuritypolicies.extensions is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "podsecuritypolicies" in API group "extensions" at the cluster scope
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=replicasets [permission-denied,permissionDenied,partialDataPreserved]: replicasets.extensions is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "replicasets" in API group "extensions" at the cluster scope
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta1, Resource=flowschemas [permission-denied,permissionDenied,partialDataPreserved]: flowschemas.flowcontrol.apiserver.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "flowschemas" in API group "flowcontrol.apiserver.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta1, Resource=prioritylevelconfigurations [permission-denied,permissionDenied,partialDataPreserved]: prioritylevelconfigurations.flowcontrol.apiserver.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "prioritylevelconfigurations" in API group "flowcontrol.apiserver.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta2, Resource=flowschemas [permission-denied,permissionDenied,partialDataPreserved]: flowschemas.flowcontrol.apiserver.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "flowschemas" in API group "flowcontrol.apiserver.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta2, Resource=prioritylevelconfigurations [permission-denied,permissionDenied,partialDataPreserved]: prioritylevelconfigurations.flowcontrol.apiserver.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "prioritylevelconfigurations" in API group "flowcontrol.apiserver.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta3, Resource=flowschemas [permission-denied,permissionDenied,partialDataPreserved]: flowschemas.flowcontrol.apiserver.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "flowschemas" in API group "flowcontrol.apiserver.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta3, Resource=prioritylevelconfigurations [permission-denied,permissionDenied,partialDataPreserved]: prioritylevelconfigurations.flowcontrol.apiserver.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "prioritylevelconfigurations" in API group "flowcontrol.apiserver.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:networking.k8s.io/v1beta1, Resource=ingressclasses [permission-denied,permissionDenied,partialDataPreserved]: ingressclasses.networking.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "ingressclasses" in API group "networking.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:networking.k8s.io/v1beta1, Resource=ingresses [permission-denied,permissionDenied,partialDataPreserved]: ingresses.networking.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "ingresses" in API group "networking.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:node.k8s.io/v1beta1, Resource=runtimeclasses [permission-denied,permissionDenied,partialDataPreserved]: runtimeclasses.node.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "runtimeclasses" in API group "node.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:policy/v1beta1, Resource=poddisruptionbudgets [permission-denied,permissionDenied,partialDataPreserved]: poddisruptionbudgets.policy is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "poddisruptionbudgets" in API group "policy" at the cluster scope
- Kubernetes: deprecated-api:policy/v1beta1, Resource=podsecuritypolicies [permission-denied,permissionDenied,partialDataPreserved]: podsecuritypolicies.policy is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "podsecuritypolicies" in API group "policy" at the cluster scope
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=clusterrolebindings [permission-denied,permissionDenied,partialDataPreserved]: clusterrolebindings.rbac.authorization.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "clusterrolebindings" in API group "rbac.authorization.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=clusterroles [permission-denied,permissionDenied,partialDataPreserved]: clusterroles.rbac.authorization.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "clusterroles" in API group "rbac.authorization.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=rolebindings [permission-denied,permissionDenied,partialDataPreserved]: rolebindings.rbac.authorization.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "rolebindings" in API group "rbac.authorization.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=roles [permission-denied,permissionDenied,partialDataPreserved]: roles.rbac.authorization.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "roles" in API group "rbac.authorization.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:scheduling.k8s.io/v1beta1, Resource=priorityclasses [permission-denied,permissionDenied,partialDataPreserved]: priorityclasses.scheduling.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "priorityclasses" in API group "scheduling.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=csidrivers [permission-denied,permissionDenied,partialDataPreserved]: csidrivers.storage.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "csidrivers" in API group "storage.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=csinodes [permission-denied,permissionDenied,partialDataPreserved]: csinodes.storage.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "csinodes" in API group "storage.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=csistoragecapacities [permission-denied,permissionDenied,partialDataPreserved]: csistoragecapacities.storage.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "csistoragecapacities" in API group "storage.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=storageclasses [permission-denied,permissionDenied,partialDataPreserved]: storageclasses.storage.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "storageclasses" in API group "storage.k8s.io" at the cluster scope
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=volumeattachments [permission-denied,permissionDenied,partialDataPreserved]: volumeattachments.storage.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "volumeattachments" in API group "storage.k8s.io" at the cluster scope
- Kubernetes: endpointslices [permission-denied,permissionDenied,partialDataPreserved]: endpointslices.discovery.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "endpointslices" in API group "discovery.k8s.io" at the cluster scope
- Kubernetes: mutatingwebhookconfigurations [permission-denied,permissionDenied,partialDataPreserved]: mutatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "mutatingwebhookconfigurations" in API group "admissionregistration.k8s.io" at the cluster scope
- Kubernetes: persistentvolumeclaims [permission-denied,permissionDenied,partialDataPreserved]: persistentvolumeclaims is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "persistentvolumeclaims" in API group "" at the cluster scope
- Kubernetes: persistentvolumes [permission-denied,permissionDenied,partialDataPreserved]: persistentvolumes is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "persistentvolumes" in API group "" at the cluster scope
- Kubernetes: poddisruptionbudgets [permission-denied,permissionDenied,partialDataPreserved]: poddisruptionbudgets.policy is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "poddisruptionbudgets" in API group "policy" at the cluster scope
- Kubernetes: pods [permission-denied,permissionDenied,partialDataPreserved]: pods is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "pods" in API group "" at the cluster scope
- Kubernetes: services [permission-denied,permissionDenied,partialDataPreserved]: services is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "services" in API group "" at the cluster scope
- Kubernetes: statefulsets [permission-denied,permissionDenied,partialDataPreserved]: statefulsets.apps is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "statefulsets" in API group "apps" at the cluster scope
- Kubernetes: validatingwebhookconfigurations [permission-denied,permissionDenied,partialDataPreserved]: validatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: User "system:serviceaccount:default:kp-cert-a" cannot list resource "validatingwebhookconfigurations" in API group "admissionregistration.k8s.io" at the cluster scope

## Upgrade Readiness

| | |
|---|---|
| **Verdict** | INCOMPLETE |
| **Readiness score** | 100/100 |
| **Coverage** | Partial |
| **Upgrade continue** | No |

> **Score interpretation:** The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score.
>
> **Advisory:** 20 applicable rules were not fully evaluated; evidence collection was incomplete for: Kubernetes. Review before approving the change.

| Category | Status | Blockers | Warnings | Rule IDs |
|---|---|---|---|---|
| API Compatibility | Passed | 0 | 0 |  |
| Extension APIs | Passed | 0 | 0 |  |
| Admission Webhooks | Passed | 0 | 0 |  |
| Disruption Safety | Passed | 0 | 0 |  |
| Drain Readiness | Passed | 0 | 0 |  |
| Node Readiness | Passed | 0 | 0 |  |
| Add-ons | Passed | 0 | 0 |  |
| CoreDNS | Passed | 0 | 0 |  |
| Workload Health | Passed | 0 | 0 |  |
| EKS Upgrade Insights | Passed | 0 | 0 |  |

## API Compatibility

| | |
|---|---|
| **Status** | Passed |
| **Upgrade continue** | Yes |
| **Score impact** | 0 |
| **Removed API objects** | 0 across 0 API families |
| **Deprecated API objects** | 0 across 0 API families |
| **Critical impact** | No |

## Evaluation Coverage

| | |
|---|---|
| **Evaluation coverage** | Partial |
| **Total rules** | 31 |
| **Evaluated** | 2 |
| **Not evaluated** | 0 |
| **Insufficient evidence** | 20 |
| **Failed** | 0 |
| **Not applicable** | 9 |
| **Source** | Native |

| Rule ID | Applicability | Execution state | Outcome | Reason |
|---|---|---|---|---|
| `ADDON-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:deployments |
| `ADDON-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:deployments |
| `API-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deprecated-api:\* |
| `API-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deprecated-api:\* |
| `APISERVICE-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:apiservices |
| `COREDNS-001` | Applicable | Evaluated | No issue detected |  |
| `CRD-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:customresourcedefinitions |
| `CRD-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:customresourcedefinitions |
| `DRAIN-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deployments, kubernetes:statefulsets |
| `DRAIN-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deployments, kubernetes:persistentvolumeclaims, kubernetes:persistentvolumes, kubernetes:pods, kubernetes:statefulsets |
| `DRAIN-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:deployments, kubernetes:pods, kubernetes:statefulsets |
| `DRAIN-004` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:pods |
| `DRAIN-005` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:statefulsets |
| `EKS-INSIGHT-001` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-INSIGHT-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-INSIGHT-003` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-001` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-003` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-004` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NET-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NODE-001` | Applicable | Evaluated | No issue detected |  |
| `NODE-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NODE-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:deployments, kubernetes:statefulsets |
| `PDB-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:poddisruptionbudgets |
| `PDB-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:poddisruptionbudgets, kubernetes:pods |
| `WH-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WH-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WH-004` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WH-005` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WORKLOAD-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:pods |
