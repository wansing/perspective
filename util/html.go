package util

import (
	"bytes"
	"io"

	"golang.org/x/net/html"
)

// AnchorHeading inserts anchor before and  "</a>" after the first heading (h1, h2, h3, h4), if any is found within the first 4000 bytes.
func AnchorHeading(input io.Reader, anchor string) io.Reader {

	tokenizer := html.NewTokenizerFragment(input, "body")
	tokenizer.SetMaxBuf(4096) // roughly the maximum number of bytes tokenized

	var modified = &bytes.Buffer{} // strings.Builder does not implement io.Reader
	var offset = 0
	var linkedTag string

	for {

		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break // assuming tokenizer.Err() == io.EOF
		}

		tagNameBytes, _ := tokenizer.TagName()
		tagName := string(tagNameBytes)

		if tt == html.StartTagToken && linkedTag == "" {
			if tagName == "h1" || tagName == "h2" || tagName == "h3" || tagName == "h4" {
				linkedTag = tagName
				modified.WriteString(anchor)
				modified.Write(tokenizer.Raw())
				continue
			}
		}

		if tt == html.EndTagToken && linkedTag != "" {
			if tagName == linkedTag {
				modified.Write(tokenizer.Raw())
				modified.WriteString(`</a>`)
				// case A: <a> and </a> have been written
				break
			}
		}

		modified.Write(tokenizer.Raw())

		offset += len(tokenizer.Raw())
		if offset > 4000 && linkedTag == "" {
			// case B: neither <a> nor </a> have been written
			break
		}
		if offset > 8000 {
			// case C: <a> has been written but </a> has not
			break
		}
	}

	return io.MultiReader(
		modified,
		bytes.NewReader(tokenizer.Buffered()), // already read from input
		input,                                 // remaining input
	)
}

// Heading returns the text of the first heading (h1, h2, h3, h4), if any is found within the first 4000 bytes.
func Heading(input io.Reader) string {

	tokenizer := html.NewTokenizerFragment(input, "body")
	tokenizer.SetMaxBuf(4096) // roughly the maximum number of bytes tokenized

	var offset = 0

	for {

		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break // assuming tokenizer.Err() == io.EOF
		}

		tagNameBytes, _ := tokenizer.TagName()
		tagName := string(tagNameBytes)

		if tt == html.StartTagToken {
			if tagName == "h1" || tagName == "h2" || tagName == "h3" || tagName == "h4" {
				return string(tokenizer.Raw())
			}
		}

		offset += len(tokenizer.Raw())
		if offset > 4000 {
			break
		}
	}

	return ""
}
