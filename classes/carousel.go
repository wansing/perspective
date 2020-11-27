package classes

import (
	"fmt"
	"os"

	"github.com/wansing/perspective/core"
)

func init() {
	Register(func() core.Class {
		return &Carousel{}
	})
}

type Carousel struct {
	Raw // provides template execution
}

func (*Carousel) Code() string {
	return "carousel"
}

func (*Carousel) Name() string {
	return "Carousel"
}

func (*Carousel) Info() string {
	return "Image carousel"
}

func (*Carousel) FeaturedChildClasses() []string {
	return nil
}

func (*Carousel) SelectOrder() core.Order {
	return core.AlphabeticallyAsc
}

type carouselData struct {
	ID    string
	Files []os.FileInfo
}

func (carousel *Carousel) Run(r *core.Query) error {

	var files, err = r.Node.Folder().Files()
	if err != nil {
		return err
	}

	var data = &carouselData{
		ID:    fmt.Sprintf("carousel-%d", r.Node.ID()),
		Files: files,
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

	// don't call t.Raw.Run, call t.Raw.ParseAndExecute with own data instead
	return carousel.Raw.ParseAndExecute(r, data)
}
