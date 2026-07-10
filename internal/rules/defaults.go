package rules

// NewDefaultRegistry returns a Registry pre-populated with every built-in
// check. New checks land here as they're implemented; nothing else in the
// scan pipeline needs to change to pick one up.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(API001{})
	r.Register(WH001{})
	r.Register(WH002{})
	r.Register(PDB001{})
	r.Register(PDB002{})
	r.Register(NODE001{})
	r.Register(NODE002{})
	r.Register(NODE003{})
	r.Register(NET002{})
	r.Register(WORKLOAD001{})
	r.Register(ADDON001{})
	r.Register(EKSNG001{})
	r.Register(EKSNG002{})
	r.Register(EKSNG003{})
	r.Register(EKSNG004{})
	r.Register(EKSINSIGHT001{})
	r.Register(EKSINSIGHT002{})
	r.Register(EKSINSIGHT003{})
	r.Register(COREDNS001{})
	r.Register(CRD001{})
	r.Register(CRD002{})
	r.Register(APIService001{})
	return r
}
