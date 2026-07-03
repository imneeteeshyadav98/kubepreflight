package findings

// FilterByNamespaceAllowlist retains cluster-scoped Kubernetes and provider
// findings, while requiring every namespaced reference in a finding to belong
// to the allowlist. A namespaced manifest with an omitted namespace is excluded
// because its eventual apply-time namespace cannot be inferred safely.
func FilterByNamespaceAllowlist(fs []Finding, namespaces []string) []Finding {
	if len(namespaces) == 0 {
		return fs
	}

	allowed := make(map[string]bool, len(namespaces))
	for _, namespace := range namespaces {
		allowed[namespace] = true
	}
	out := make([]Finding, 0, len(fs))
	for _, f := range fs {
		include := true
		for _, ref := range f.Resources {
			if ref.Scope != ScopeNamespaced {
				continue
			}
			if ref.Namespace == "" || !allowed[ref.Namespace] {
				include = false
				break
			}
		}
		if include {
			out = append(out, f)
		}
	}
	return out
}
