package classes

import (
	"sort"

	"github.com/wansing/perspective/core"
)

// implements core.ClassRegistry
type Registry map[string]*core.Class

func (reg Registry) Add(class *core.Class) {
	reg[class.Code] = class
}

func (reg Registry) All() []string {
	var all = make([]string, 0, len(reg))
	for code := range reg {
		all = append(all, code)
	}
	sort.Strings(all)
	return all
}

func (reg Registry) Get(code string) (*core.Class, bool) {
	class, ok := reg[code]
	return class, ok
}

var DefaultRegistry = make(Registry)

func Register(class *core.Class) {
	DefaultRegistry.Add(class)
}
