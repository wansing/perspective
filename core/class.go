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

// A class defines the behavior of a node.
type Class struct {
	Create               func() Instance
	Name                 string
	Code                 string
	Info                 string
	SelectOrder          Order    // for backend select
	FeaturedChildClasses []string // for backend create
}

func (class *Class) InfoHTML() template.HTML {
	return template.HTML(class.Info)
}

// An instance of a class can store request-scoped data.
type Instance interface {
	AddSlugs() []string // is called after Do
	Do(*Route) error
}

type NOP struct{}

func (t *NOP) AddSlugs() []string {
	return nil
}

// Do won't reveal any content to the viewer.
func (t *NOP) Do(r *Route) error {
	return r.Recurse()
}
