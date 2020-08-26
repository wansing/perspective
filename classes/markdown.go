package classes

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/wansing/perspective/core"
	"gitlab.com/golang-commonmark/markdown"
)

var markdownParser *markdown.Markdown = markdown.New(markdown.HTML(true), markdown.Linkify(true), markdown.Typographer(true), markdown.MaxNesting(10))

func init() {
	Register(&core.Class{
		Create: func() core.Instance {
			return &Markdown{}
		},
		Name: "Markdown document",
		Code: "markdown",
		Info: `
			<p>Translates <a href="https://spec.commonmark.org/0.28/">CommonMark Markdown</a> to HTML.</p>
			<h4>Examples</h4>
			<table class="table table-sm">
				<tbody>
					<tr>
						<td>two line breaks</td>
						<td>new paragraph</td>
					</tr>
					<tr>
						<td><code># Heading</code></td>
						<td>top level heading</td>
					</tr>
					<tr>
						<td><code>## Heading</code></td>
						<td>second level heading</td>
					</tr>
					<tr>
						<td><code>*example*</code></td>
						<td><em>example</em></td>
					</tr>
					<tr>
						<td><code>**example**</code></td>
						<td><strong>example</strong></td>
					</tr>
					<tr>
						<td><code>* example</code></td>
						<td>unordered list</td>
					</tr>
					<tr>
						<td><code>1. example</code></td>
						<td>ordered list</td>
					</tr>
					<tr>
						<td><code>&gt; example</code></td>
						<td>quotation</td>
					</tr>
					<tr>
						<td><code>[click here](https://www.example.com)</code></td>
						<td>link</td>
					</tr>
				</tbody>
			</table>`,
	})
}

type Markdown struct {
	Html
}

// Where should markdown rendering happen? The data flow is:
//
// 1. content string, the best place to render markdown
// 2. template parsing, rendering markdown would tear apart things like "# Section {{.Name}}", and godoc says the parse tree "should be treated as unexported by all other clients"
// 3. template execution, does recursion, returns strings, rendering markdown would process next and included nodes as well

func (t *Markdown) OnPrepare(r *core.Route) error {

	// remove all tabs from the beginning of each line

	var unindentedContent = &bytes.Buffer{}

	lineScanner := bufio.NewScanner(strings.NewReader(t.Node.Content()))
	for lineScanner.Scan() {
		line := lineScanner.Text()
		for len(line) > 0 && line[0] == '\t' {
			line = line[1:]
		}
		unindentedContent.WriteString(line)
		unindentedContent.WriteString("\n")
	}

	// render markdown

	var renderedMarkdown = markdownParser.RenderToString(unindentedContent.Bytes())

	// restore quotation marks within template instructions {{ }}

	var result = &bytes.Buffer{}

	var inBraces = false
	var prevRune rune = 0
	for _, r := range renderedMarkdown {
		if r == '{' && prevRune == '{' {
			inBraces = true
		}
		if r == '}' && prevRune == '}' {
			inBraces = false
		}
		if inBraces && (r == '„' || r == '“' || r == '”') { // these take more than one byte, so we can't replace them in place
			result.WriteByte('"')
		} else {
			result.WriteRune(r)
		}
		prevRune = r
	}

	t.Node.SetContent(result.String())

	// this func shadows Html.OnPrepare, so we call it now
	return t.Html.OnPrepare(r)
}
