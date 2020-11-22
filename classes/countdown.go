package classes

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"time"

	"github.com/icza/gox/timex"
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
		Info: `<p>Countdown respects leap years and daylight saving time. Use it like this:</p>

<pre><code>{{.T.SetEnd "1 Jan 2100 12:00:00 -0100"}}
{{.T.Years}} years {{.T.Months}} months {{.T.Days}} days {{.T.Hours}} hours {{.T.Minutes}} minutes {{.T.Seconds}} seconds left
</code></pre>`,
	})
}

type Countdown struct {
	Raw         // for template execution
	End         time.Time
	CountdownID string // random string, so multiple instances of countdown won't collide
	Years       template.HTML
	Months      template.HTML
	Days        template.HTML
	Hours       template.HTML
	Minutes     template.HTML
	Seconds     template.HTML
}

func (t *Countdown) SetEnd(endStr string) error {
	var end, err = time.Parse("_2 Jan 2006 15:04:05 -0700", endStr)
	if err != nil {
		return err
	}

	var years, months, days, hours, minutes, seconds = timex.Diff(time.Now(), end) // timex respect leap years

	t.End = end
	t.Years = template.HTML(fmt.Sprintf(`<span id="years-%s">%d</span>`, t.CountdownID, years))
	t.Months = template.HTML(fmt.Sprintf(`<span id="months-%s">%d</span>`, t.CountdownID, months))
	t.Days = template.HTML(fmt.Sprintf(`<span id="days-%s">%d</span>`, t.CountdownID, days))
	t.Hours = template.HTML(fmt.Sprintf(`<span id="hours-%s">%02d</span>`, t.CountdownID, hours))
	t.Minutes = template.HTML(fmt.Sprintf(`<span id="minutes-%s">%02d</span>`, t.CountdownID, minutes))
	t.Seconds = template.HTML(fmt.Sprintf(`<span id="seconds-%s">%02d</span>`, t.CountdownID, seconds))
	return nil
}

func (t *Countdown) CountdownIDJS() template.JS {
	return template.JS(t.CountdownID)
}

func (t *Countdown) Do(r *core.Route) error {

	var countdownID = make([]byte, 6)
	if _, err := rand.Read(countdownID); err != nil {
		return err
	}
	t.CountdownID = fmt.Sprintf("c%X", countdownID) // hexadecimal, start with a character so digits won't look like subtraction

	r.SetContent(
		`{{define "head"}}
			{{.Get "head"}}

			<script type="text/javascript">

				function pad(n) {
					if(n >= 0 && n < 10) {
						return "0" + n;
					}
					return n;
				}

				function countdown{{.T.CountdownIDJS}}() {

					// inspired from https://github.com/icza/gox/blob/master/timex/timex.go
					// assuming both are in the same location

					let a = new Date();
					let b = new Date({{.T.End.Unix}} * 1000); // constructor takes milliseconds

					if(a > b) {
						console.log("too late ", a, b);
						return;
					}

					let y1 = a.getFullYear();
					let M1 = a.getMonth();
					let d1 = a.getDate();

					let y2 = b.getFullYear();
					let M2 = b.getMonth();
					let d2 = b.getDate();

					let h1 = a.getHours();
					let m1 = a.getMinutes();
					let s1 = a.getSeconds();

					let h2 = b.getHours();
					let m2 = b.getMinutes();
					let s2 = b.getSeconds();

					let years = y2 - y1;
					let months = M2 - M1;
					let days = d2 - d1;
					let hours = h2 - h1;
					let mins = m2 - m1;
					let secs = s2 - s1;

					if(secs < 0) {
						secs += 60;
						mins--;
					}

					if(mins < 0) {
						mins += 60;
						hours--;
					}

					if(hours < 0) {
						hours += 24;
						days--;
					}

					if(days < 0) {
						let t = new Date(y1, M1, 32, 0, 0, 0);
						days += 32 - t.getDate();
						months--;
					}

					if(months < 0) {
						months += 12;
						years--;
					}

					elementYears = document.getElementById("years-{{.T.CountdownID}}");
					if(elementYears) {
						elementYears.innerHTML = years;
					}

					elementMonths = document.getElementById("months-{{.T.CountdownID}}");
					if(elementMonths) {
						elementMonths.innerHTML = months;
					}

					elementDays = document.getElementById("days-{{.T.CountdownID}}");
					if(elementDays) {
						elementDays.innerHTML = days;
					}

					elementHours = document.getElementById("hours-{{.T.CountdownID}}");
					if(elementHours) {
						elementHours.innerHTML = pad(hours);
					}

					elementMinutes = document.getElementById("minutes-{{.T.CountdownID}}");
					if(elementMinutes) {
						elementMinutes.innerHTML = pad(mins);
					}

					elementSeconds = document.getElementById("seconds-{{.T.CountdownID}}");
					if(elementSeconds) {
						elementSeconds.innerHTML = pad(secs);
					}

					setTimeout(countdown{{.T.CountdownIDJS}}, 1000);
				}

				countdown{{.T.CountdownIDJS}}();

			</script>
		{{end}}` + r.Content())

	return t.Raw.Do(r)
}
