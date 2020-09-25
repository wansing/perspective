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
	Create               func() Instance // for initialization work only
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
	AdditionalSlugs() []string // should be called after Do only
	Do(*Route) error
}
