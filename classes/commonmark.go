package classes

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/wansing/perspective/core"
	"gitlab.com/golang-commonmark/markdown"
)

var commonMarkParser *markdown.Markdown = markdown.New(markdown.HTML(true), markdown.Linkify(true), markdown.Typographer(true), markdown.MaxNesting(10))

func init() {
	Register(func() core.Class {
		return &CommonMark{}
	})
}

// CommonMark renders the content as CommonMark markdown and runs HTML.
//
// This order is crucial. If templates were processed first, then embedded content would be rendered as well.
// Now instead, markdown rendering must take care to skip template instructions.
type CommonMark struct {
	HTML
}

func (CommonMark) Code() string {
	return "commonmark"
}

func (CommonMark) Name() string {
	return "Markdown document (CommonMark)"
}

func (CommonMark) Info() string {
	return `<p>Translates <a href="https://spec.commonmark.org/0.28/">CommonMark Markdown</a> to HTML.</p>
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
			</table>`
}

func (CommonMark) FeaturedChildClasses() []string {
	return nil
}

func (CommonMark) SelectOrder() core.Order {
	return core.AlphabeticallyAsc
}

func (md CommonMark) Run(r *core.Query) error {

	rendered := renderMarkdown(strings.NewReader(r.Content()))
	r.SetContent(rendered)

	return md.HTML.Run(r)
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

	var tokens = commonMarkParser.Parse(unindentedContent.Bytes())
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
	commonMarkParser.RenderTokens(result, tokens)
	return result.String()
}
