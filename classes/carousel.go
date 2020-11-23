package classes

import (
	"os"
	"strconv"

	"github.com/wansing/perspective/core"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Carousel{}
		},
		Name: "Carousel",
		Code: "carousel",
		Info: `Image carousel`,
	})
}

type Carousel struct {
	CarouselID string
	Files      []os.FileInfo
}

func (t *Carousel) AddSlugs() []string {
	return nil
}

func (t *Carousel) Do(r *core.Route) error {

	t.CarouselID = "carousel-" + strconv.Itoa(r.Node.ID())

	var err error
	t.Files, err = r.Node.Folder().Files()
	if err != nil {
		return err
	}

	r.SetGlobal("include-bootstrap-4-css", "true")
	r.SetGlobal("include-bootstrap-4-js", "true")
	r.SetGlobal("include-jquery-3", "true")

	// ignore existing content
	r.SetContent(
		`<div id="{{ .CarouselID }}" class="carousel slide" data-ride="carousel">
			<ol class="carousel-indicators">
				{{ range $index, $file := .Files }}
					<li data-target="#{{ $.CarouselID }}" data-slide-to="{{ $index }}"></li>
				{{ end }}
			</ol>
			<div class="carousel-inner">
				{{ range $index, $file := .Files }}
					<div class="carousel-item{{ if eq $index 0 }} active{{ end }}"><img class="d-block w-100" src="/upload/{{ $.Node.ID }}/{{ $file.Name }}"></div>
				{{ end }}
			</div>
			<a class="carousel-control-prev" href="#{{ .CarouselID }}" role="button" data-slide="prev">
				<span class="carousel-control-prev-icon" aria-hidden="true"></span>
				<span class="sr-only">Previous</span>
			</a>
			<a class="carousel-control-next" href="#{{ .CarouselID }}" role="button" data-slide="next">
				<span class="carousel-control-next-icon" aria-hidden="true"></span>
				<span class="sr-only">Next</span>
			</a>
		</div>`)

	return r.Recurse()
}
