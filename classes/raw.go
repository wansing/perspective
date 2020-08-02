package classes

import "github.com/wansing/perspective/core"

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Raw{}
		},
		Name: "Raw HTML document",
		Code: "raw",
	})
}

type Raw struct {
	core.Base
}
