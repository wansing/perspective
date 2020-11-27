package classes

import (
	"net/url"
	"path"

	"github.com/wansing/perspective/core"
)

func init() {
	Register(func() core.Class {
		return &Redirect{}
	})
}

type Redirect struct{}

func (Redirect) Code() string {
	return "redirect"
}

func (Redirect) Name() string {
	return "Redirect"
}

func (Redirect) Info() string {
	return ""
}

func (Redirect) FeaturedChildClasses() []string {
	return nil
}

func (Redirect) SelectOrder() core.Order {
	return core.AlphabeticallyAsc
}

func (Redirect) Run(r *core.Query) error {

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
