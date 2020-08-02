package util

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RandomString32 returns a 32 bytes long string with 24 bytes (192 bits) of entropy.
func RandomString32() (string, error) {

	b := make([]byte, 24)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	result := base64.URLEncoding.EncodeToString(b)

	if len(result) < 32 {
		return "", errors.New("RandomString64 too short")
	}

	if len(result) > 32 {
		result = result[:32]
	}

	return result, nil
}

// Trunc truncates the input string to a specific length.
// It is UTF8-safe, but does not care for HTML.
func Trunc(input string, maxRunes int) string {
	input = strings.TrimSpace(input)
	var runes = 0
	for i := range input {
		runes++
		if runes == maxRunes {
			return strings.TrimSpace(input[:i]) // trim spaces again
		}
	}
	return input
}

// ParseTime parses a string like "02.01.2006 15:04" to a unix timestamp.
func ParseTime(ts string) (int64, error) {
	t, err := time.ParseInLocation("02.01.2006 15:04", ts, time.Local)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

// Pages returns non-consecutive page numbers from 1 to numPages.
func Pages(currentPage int, numPages int) []int {

	// collect page numbers in a map

	pages := map[int]interface{}{}

	pages[1] = struct{}{}
	pages[currentPage] = struct{}{}
	pages[numPages] = struct{}{}

	delta := 1 // Differenz zu currentPage
	watchdog := 1

	for (currentPage-delta > 1 || currentPage+delta < numPages) && watchdog < 20 {

		if currentPage-delta > 0 {
			pages[currentPage-delta] = struct{}{}
		}

		if currentPage+delta < numPages {
			pages[currentPage+delta] = struct{}{}
		}

		delta *= 2
		watchdog++
	}

	// map to slice

	pageslice := []int{}

	for page := range pages { // map: page -> interface{}
		pageslice = append(pageslice, page)
	}

	sort.Ints(pageslice)

	return pageslice
}

// PageLinks calls Pages and wraps links around its result.
func PageLinks(currentPage int, numPages int, htm func(page int, name string) string, currentPageHtm func(page int, name string) string) []template.HTML {

	pagelinks := []template.HTML{}

	if currentPage < 1 || numPages < 1 {
		return pagelinks
	}

	pagenumbers := Pages(currentPage, numPages)

	if currentPage > 1 {
		pagelinks = append(pagelinks, template.HTML(htm(currentPage-1, `&laquo;`)))
	}

	for _, page := range pagenumbers {

		if page == currentPage {
			pagelinks = append(pagelinks, template.HTML(currentPageHtm(page, strconv.Itoa(page))))
		} else {
			pagelinks = append(pagelinks, template.HTML(htm(page, strconv.Itoa(page))))
		}
	}

	if currentPage < numPages {
		pagelinks = append(pagelinks, template.HTML(htm(currentPage+1, `&raquo;`)))
	}

	return pagelinks
}
