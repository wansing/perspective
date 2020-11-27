package classes

import (
	"sort"

	"github.com/wansing/perspective/core"
)

// implements core.ClassRegistry
type Registry map[string]func() core.Class

func (reg Registry) Add(f func() core.Class) {
	reg[f().Code()] = f
}

func (reg Registry) All() []string {
	var all = make([]string, 0, len(reg))
	for code := range reg {
		all = append(all, code)
	}
	sort.Strings(all)
	return all
}

func (reg Registry) Get(code string) (core.Class, bool) {
	if f, ok := reg[code]; ok {
		return f(), true
	}
	return nil, false
}

var DefaultRegistry = make(Registry)

func Register(f func() core.Class) {
	DefaultRegistry.Add(f)
}
