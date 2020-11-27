package classes

import (
	"net/url"
	"path"

	"github.com/wansing/perspective/core"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Redirect{}
		},
		Name: "Redirect",
		Code: "redirect",
	})
}

type Redirect struct{}

func (t *Redirect) AddSlugs() []string {
	return nil
}

func (t *Redirect) Do(r *core.Query) error {

	u, err := url.Parse(r.Version.Content())
	if err != nil {
		return err
	}

	if !u.IsAbs() && !path.IsAbs(u.Path) {
		u.Path = path.Join(r.Request.Path, u.Path)
	}

	r.SeeOther(u.String())
	// no recursion
	return nil
}
