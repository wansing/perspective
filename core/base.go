package core

// Instances can embed the Base class or just implement both functions.
type Base struct{}

func (t *Base) AdditionalSlugs() []string {
	return nil
}

func (t *Base) Do(r *Route) error {
	return r.Recurse()
}
