package core

type Order int

const (
	AlphabeticallyAsc Order = iota
	ChronologicallyDesc
)

type ClassRegistry interface {
	All() []string
	Get(code string) (Class, bool)
}

// A class executes a node.
//
// Class is an interface and not a struct, so it can't be modified.
type Class interface {
	Run(*Query) error
	Code() string
	Name() string
	Info() string
	FeaturedChildClasses() []string // for backend create
	SelectOrder() Order             // for backend select
}

type UnknownClass string

// Run won't reveal any content to the viewer.
func (UnknownClass) Run(r *Query) error {
	return r.Recurse()
}

func (class UnknownClass) Code() string {
	return string(class)
}

func (UnknownClass) Name() string {
	return "Unknown class"
}

func (UnknownClass) Info() string {
	return ""
}

func (UnknownClass) FeaturedChildClasses() []string {
	return nil
}

func (UnknownClass) SelectOrder() Order {
	return AlphabeticallyAsc
}
