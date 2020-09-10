package core

import (
	"html/template"
)

type Order int

const (
	AlphabeticallyAsc Order = iota
	ChronologicallyDesc
)

type ClassRegistry interface {
	All() []string
	Get(code string) (*Class, bool)
}

type Class struct {
	Create               func() Instance
	Name                 string
	Code                 string
	Info                 string
	SelectOrder          Order // for backend select
	FeaturedChildClasses []string
}

func (class *Class) InfoHTML() template.HTML {
	return template.HTML(class.Info)
}

// An Instance of a Class is wrapped around an Node.
type Instance interface {
	AdditionalSlugs() []string
	GetNode() *Node // classes can access Foo.Base.Node directly, but the core has just a Class object and must use this method to get the *Node
	SetNode(*Node)
	Do(*Route) error
}

// All Instances should embed the Base class.
type Base struct {
	*Node
}

func (t *Base) AdditionalSlugs() []string {
	return nil
}

func (t *Base) GetNode() *Node {
	return t.Node
}

func (t *Base) SetNode(node *Node) {
	t.Node = node
}

func (t *Base) Do(r *Route) error {
	return r.Recurse()
}
