package rules

// NewDefaultRegistry returns a Registry pre-populated with every built-in
// check. New checks land here as they're implemented; nothing else in the
// scan pipeline needs to change to pick one up.
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(API001{})
	r.Register(API002{})
	r.Register(WH001{})
	r.Register(WH002{})
	r.Register(PDB001{})
	r.Register(PDB002{})
	r.Register(NODE001{})
	r.Register(NODE002{})
	r.Register(NET002{})
	r.Register(ADDON001{})
	r.Register(COREDNS001{})
	r.Register(CRD001{})
	r.Register(CRD002{})
	r.Register(APIService001{})
	return r
}
