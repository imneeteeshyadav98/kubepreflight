# KubePreflight Scan Report

| | |
|---|---|
| **Cluster** | unreachable |
| **Target version** | 1.34 |
| **Provider** | cluster-only |
| **Upgrade context** | unspecified |
| **Scanned at** | 2026-07-29 07:32:13 UTC |
| **Result** | **INCOMPLETE** |
| **Summary** | 0 blocker(s), 0 warning(s), 0 operator decision(s), 0 info(s) |

> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.

- Kubernetes: apiservices [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apiregistration.k8s.io/v1/apiservices": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: coredns-configmap [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/api/v1/namespaces/kube-system/configmaps/coredns": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: customresourcedefinitions [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apiextensions.k8s.io/v1/customresourcedefinitions": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: daemonsets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1/daemonsets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deployments [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1/deployments": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:admissionregistration.k8s.io/v1beta1, Resource=mutatingwebhookconfigurations [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/admissionregistration.k8s.io/v1beta1/mutatingwebhookconfigurations": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:admissionregistration.k8s.io/v1beta1, Resource=validatingwebhookconfigurations [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/admissionregistration.k8s.io/v1beta1/validatingwebhookconfigurations": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apiextensions.k8s.io/v1beta1, Resource=customresourcedefinitions [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apiregistration.k8s.io/v1beta1, Resource=apiservices [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apiregistration.k8s.io/v1beta1/apiservices": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apps/v1beta1, Resource=deployments [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1beta1/deployments": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apps/v1beta1, Resource=statefulsets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1beta1/statefulsets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apps/v1beta2, Resource=daemonsets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1beta2/daemonsets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apps/v1beta2, Resource=deployments [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1beta2/deployments": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apps/v1beta2, Resource=replicasets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1beta2/replicasets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:apps/v1beta2, Resource=statefulsets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1beta2/statefulsets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:autoscaling/v2beta1, Resource=horizontalpodautoscalers [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/autoscaling/v2beta1/horizontalpodautoscalers": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:autoscaling/v2beta2, Resource=horizontalpodautoscalers [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/autoscaling/v2beta2/horizontalpodautoscalers": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:batch/v1beta1, Resource=cronjobs [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/batch/v1beta1/cronjobs": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:certificates.k8s.io/v1beta1, Resource=certificatesigningrequests [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/certificates.k8s.io/v1beta1/certificatesigningrequests": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:coordination.k8s.io/v1beta1, Resource=leases [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/coordination.k8s.io/v1beta1/leases": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:discovery.k8s.io/v1beta1, Resource=endpointslices [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/discovery.k8s.io/v1beta1/endpointslices": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:events.k8s.io/v1beta1, Resource=events [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/events.k8s.io/v1beta1/events": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=daemonsets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/extensions/v1beta1/daemonsets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=deployments [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/extensions/v1beta1/deployments": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=ingresses [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/extensions/v1beta1/ingresses": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=networkpolicies [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/extensions/v1beta1/networkpolicies": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=podsecuritypolicies [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/extensions/v1beta1/podsecuritypolicies": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:extensions/v1beta1, Resource=replicasets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/extensions/v1beta1/replicasets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta1, Resource=flowschemas [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/flowcontrol.apiserver.k8s.io/v1beta1/flowschemas": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta1, Resource=prioritylevelconfigurations [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/flowcontrol.apiserver.k8s.io/v1beta1/prioritylevelconfigurations": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta2, Resource=flowschemas [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/flowcontrol.apiserver.k8s.io/v1beta2/flowschemas": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta2, Resource=prioritylevelconfigurations [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/flowcontrol.apiserver.k8s.io/v1beta2/prioritylevelconfigurations": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta3, Resource=flowschemas [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/flowcontrol.apiserver.k8s.io/v1beta3/flowschemas": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:flowcontrol.apiserver.k8s.io/v1beta3, Resource=prioritylevelconfigurations [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/flowcontrol.apiserver.k8s.io/v1beta3/prioritylevelconfigurations": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:networking.k8s.io/v1beta1, Resource=ingressclasses [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/networking.k8s.io/v1beta1/ingressclasses": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:networking.k8s.io/v1beta1, Resource=ingresses [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/networking.k8s.io/v1beta1/ingresses": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:node.k8s.io/v1beta1, Resource=runtimeclasses [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/node.k8s.io/v1beta1/runtimeclasses": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:policy/v1beta1, Resource=poddisruptionbudgets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/policy/v1beta1/poddisruptionbudgets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:policy/v1beta1, Resource=podsecuritypolicies [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/policy/v1beta1/podsecuritypolicies": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=clusterrolebindings [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/rbac.authorization.k8s.io/v1beta1/clusterrolebindings": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=clusterroles [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/rbac.authorization.k8s.io/v1beta1/clusterroles": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=rolebindings [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/rbac.authorization.k8s.io/v1beta1/rolebindings": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:rbac.authorization.k8s.io/v1beta1, Resource=roles [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/rbac.authorization.k8s.io/v1beta1/roles": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:scheduling.k8s.io/v1beta1, Resource=priorityclasses [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/scheduling.k8s.io/v1beta1/priorityclasses": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=csidrivers [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/storage.k8s.io/v1beta1/csidrivers": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=csinodes [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/storage.k8s.io/v1beta1/csinodes": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=csistoragecapacities [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/storage.k8s.io/v1beta1/csistoragecapacities": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=storageclasses [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/storage.k8s.io/v1beta1/storageclasses": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: deprecated-api:storage.k8s.io/v1beta1, Resource=volumeattachments [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/storage.k8s.io/v1beta1/volumeattachments": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: endpointslices [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/discovery.k8s.io/v1/endpointslices": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: mutatingwebhookconfigurations [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/admissionregistration.k8s.io/v1/mutatingwebhookconfigurations": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: nodes [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/api/v1/nodes": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: persistentvolumeclaims [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/api/v1/persistentvolumeclaims": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: persistentvolumes [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/api/v1/persistentvolumes": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: poddisruptionbudgets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/policy/v1/poddisruptionbudgets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: pods [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/api/v1/pods": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: services [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/api/v1/services": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: statefulsets [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/apps/v1/statefulsets": dial tcp 127.0.0.1:9: socket: operation not permitted
- Kubernetes: validatingwebhookconfigurations [unknown-runtime-failure,partialDataPreserved]: Get "https://127.0.0.1:9/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations": dial tcp 127.0.0.1:9: socket: operation not permitted

## Upgrade Readiness

| | |
|---|---|
| **Verdict** | INCOMPLETE |
| **Readiness score** | 100/100 |
| **Coverage** | Partial |
| **Upgrade continue** | No |

> **Score interpretation:** The readiness score is based on findings produced by evaluated checks. Rules that were not evaluated are not penalized in the score.
>
> **Advisory:** 22 applicable rules were not fully evaluated; evidence collection was incomplete for: Kubernetes. Review before approving the change.

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
| **Evaluated** | 0 |
| **Not evaluated** | 0 |
| **Insufficient evidence** | 22 |
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
| `COREDNS-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:coredns-configmap |
| `CRD-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:customresourcedefinitions |
| `CRD-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:customresourcedefinitions |
| `DRAIN-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deployments, kubernetes:statefulsets |
| `DRAIN-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:deployments, kubernetes:nodes, kubernetes:persistentvolumeclaims, kubernetes:persistentvolumes, kubernetes:pods, kubernetes:statefulsets |
| `DRAIN-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:deployments, kubernetes:nodes, kubernetes:pods, kubernetes:statefulsets |
| `DRAIN-004` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:nodes, kubernetes:pods |
| `DRAIN-005` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:statefulsets |
| `EKS-INSIGHT-001` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-INSIGHT-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-INSIGHT-003` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-001` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-003` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `EKS-NG-004` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NET-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NODE-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:nodes |
| `NODE-002` | Not applicable | Not evaluated | Not applicable | required evidence plane was not requested for this scan mode |
| `NODE-003` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:daemonsets, kubernetes:deployments, kubernetes:statefulsets |
| `PDB-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:poddisruptionbudgets |
| `PDB-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:poddisruptionbudgets, kubernetes:pods |
| `WH-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WH-002` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WH-004` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WH-005` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:mutatingwebhookconfigurations, kubernetes:validatingwebhookconfigurations |
| `WORKLOAD-001` | Applicable | Insufficient evidence | Unable to verify | required collector data was unavailable for: kubernetes:pods |
