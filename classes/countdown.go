package classes

// ignores daylight saving time

import (
	"errors"
	"fmt"
	"html/template"
	"strconv"
	"time"

	"github.com/wansing/perspective/core"
)

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Countdown{}
		},
		Name: "Countdown",
		Code: "countdown",
		Info: `TODO`,
	})
}

type Countdown struct {
	core.Base
	EndUnix int64
}

func (t *Countdown) UniqueId() string {
	return strconv.Itoa(t.Id()) // t.Id() == t.Base.Node.DBNode.Id()
}

func (t *Countdown) UniqueIdJS() template.JS {
	return template.JS(t.UniqueId())
}

func (t *Countdown) OnPrepare(r *core.Route) error {

	// get local variable

	endStr := r.GetLocalStr("endtime")
	if endStr == "" {
		endStr = "1 Jan 2100 00:00:00 -0000"
	}

	end, err := time.Parse("_2 Jan 2006 15:04:05 -0700", endStr)
	if err != nil {
		return errors.New("could not parse endtime") // TODO not shown atm?
	}

	t.EndUnix = end.Unix()

	// set local variables

	var days int64
	var hours int64
	var minutes int64
	var seconds int64

	diff := t.EndUnix - time.Now().Unix()
	if diff < 0 {
		diff = 0
	}

	days = diff / 86400
	diff %= 86400
	hours = diff / 3600
	diff %= 3600
	minutes = diff / 60
	seconds = diff % 60

	r.SetLocal("days", fmt.Sprintf(`<span id="days-%s">%02d</span>`, t.UniqueId(), days))
	r.SetLocal("hours", fmt.Sprintf(`<span id="hours-%s">%02d</span>`, t.UniqueId(), hours))
	r.SetLocal("minutes", fmt.Sprintf(`<span id="minutes-%s">%02d</span>`, t.UniqueId(), minutes))
	r.SetLocal("seconds", fmt.Sprintf(`<span id="seconds-%s">%02d</span>`, t.UniqueId(), seconds))

	r.Current().SetContent(
		`{{define "head"}}
			{{.Get "head"}}

			<script type="text/javascript">

				function pad(n) {
					if(n >= 0 && n < 10) {
						return "0" + n;
					}
					return n;
				}

				function countdown{{.T.UniqueIdJS}}() {

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

					elementDays = document.getElementById("days-{{.T.UniqueId}}");
					if(elementDays) {
						elementDays.innerHTML = days;
					}

					elementHours = document.getElementById("hours-{{.T.UniqueId}}");
					if(elementHours) {
						elementHours.innerHTML = pad(hours);
					}

					elementMinutes = document.getElementById("minutes-{{.T.UniqueId}}");
					if(elementMinutes) {
						elementMinutes.innerHTML = pad(minutes);
					}

					elementSeconds = document.getElementById("seconds-{{.T.UniqueId}}");
					if(elementSeconds) {
						elementSeconds.innerHTML = pad(seconds);
					}

					setTimeout(countdown{{.T.UniqueIdJS}}, 1000);
				}

				countdown{{.T.UniqueIdJS}}();

			</script>
		{{end}}` + r.Current().Content())

	return nil
}
