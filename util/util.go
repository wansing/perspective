package util

import (
	"html/template"
	"sort"
	"strconv"
)

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
