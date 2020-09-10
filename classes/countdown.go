package classes

// ignores daylight saving time

import (
	"fmt"
	"html/template"
	"time"

	"github.com/wansing/perspective/core"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Countdown{
				End: time.Now(),
			}
		},
		Name: "Countdown",
		Code: "countdown",
		Info: `TODO`,
	})
}

type Countdown struct {
	Raw // required for template execution
	End time.Time
}

func (t *Countdown) IdJS() template.JS {
	return template.JS(t.Node.Id())
}

func (t *Countdown) Days() template.HTML {
	return template.HTML(fmt.Sprintf(`<span id="days-%s">%02d</span>`, t.Node.Id(), t.End.Sub(time.Now()).Hours()/24))
}

func (t *Countdown) Hours() template.HTML {
	return template.HTML(fmt.Sprintf(`<span id="hours-%s">%02d</span>`, t.Node.Id(), t.End.Sub(time.Now()).Hours()))
}

func (t *Countdown) Minutes() template.HTML {
	return template.HTML(fmt.Sprintf(`<span id="minutes-%s">%02d</span>`, t.Node.Id(), t.End.Sub(time.Now()).Minutes()))
}

func (t *Countdown) Seconds() template.HTML {
	return template.HTML(fmt.Sprintf(`<span id="seconds-%s">%02d</span>`, t.Node.Id(), t.End.Sub(time.Now()).Seconds()))
}

func (t *Countdown) SetEnd(end string) (err error) {
	t.End, err = time.Parse("_2 Jan 2006 15:04:05 -0700", end)
	return
}

func (t *Countdown) Do(r *core.Route) error {

	t.Node.SetContent(
		`{{define "head"}}
			{{.Get "head"}}

			<script type="text/javascript">

				function pad(n) {
					if(n >= 0 && n < 10) {
						return "0" + n;
					}
					return n;
				}

				function countdown{{.T.IdJS}}() {

					var end = {{.T.EndUnix}};
					var now = Math.floor(new Date().getTime() / 1000);
					var diff = end - now;

					if(diff < 0) {
						return;
					}

					var days = Math.floor(diff / 86400);
					diff = diff % 86400;
					var hours = Math.floor(diff / 3600);
					diff = diff % 3600;
					var minutes = Math.floor(diff / 60);
					var seconds = diff % 60;

					elementDays = document.getElementById("days-{{.T.Node.Id}}");
					if(elementDays) {
						elementDays.innerHTML = days;
					}

					elementHours = document.getElementById("hours-{{.T.Node.Id}}");
					if(elementHours) {
						elementHours.innerHTML = pad(hours);
					}

					elementMinutes = document.getElementById("minutes-{{.T.Node.Id}}");
					if(elementMinutes) {
						elementMinutes.innerHTML = pad(minutes);
					}

					elementSeconds = document.getElementById("seconds-{{.T.Node.Id}}");
					if(elementSeconds) {
						elementSeconds.innerHTML = pad(seconds);
					}

					setTimeout(countdown{{.T.IdJS}}, 1000);
				}

				countdown{{.T.IdJS}}();

			</script>
		{{end}}` + t.Node.Content())

	return t.Raw.Do(r)
}
