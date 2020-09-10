package classes

import (
	"bufio"
	"bytes"
	"io"
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

// Do renders the content as markdown and then calls HTML.Do().
//
// The order is crucial.
// If templates were processed first, then embedded content would be rendered as well.
// Now instead, markdown rendering must take care to skip template instructions.
type Markdown struct {
	HTML
}

func (t *Markdown) Do(r *core.Route) error {

	rendered := renderMarkdown(strings.NewReader(t.Node.Content()))
	t.Node.SetContent(rendered)

	return t.HTML.Do(r)
}

func renderMarkdown(input io.Reader) string {

	// remove all tabs from the beginning of each line

	var unindentedContent = &bytes.Buffer{}

	lineScanner := bufio.NewScanner(input)
	for lineScanner.Scan() {
		line := lineScanner.Text()
		for len(line) > 0 && line[0] == '\t' {
			line = line[1:]
		}
		unindentedContent.WriteString(line)
		unindentedContent.WriteString("\n")
	}

	// render markdown

	var tokens = markdownParser.Parse(unindentedContent.Bytes())
	for i, t := range tokens {
		if inline, ok := t.(*markdown.Inline); ok {
			if strings.HasPrefix(inline.Content, "{{") && strings.HasSuffix(inline.Content, "}}") {
				tokens[i] = &markdown.Text{
					Content: inline.Content,
					Lvl:     inline.Level(),
				}
			}
		}
	}

	var result = &bytes.Buffer{}
	markdownParser.RenderTokens(result, tokens)
	return result.String()
}
