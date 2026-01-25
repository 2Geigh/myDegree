package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gocolly/colly"
)

type CourseInfo struct {
	faculty     string
	name        string
	description string
	prereqs     []string
}

var (
	Courses map[string]CourseInfo = make(map[string]CourseInfo)
)

func main() {
	wg := sync.WaitGroup{}

	wg.Add(1)
	go getAllCourses(&wg)

	wg.Wait()
}

func getAllCourses(wg *sync.WaitGroup) {
	defer wg.Done()

	c := colly.NewCollector(
		colly.AllowedDomains("artsci.calendar.utoronto.ca"),
	)

	// Get courses from each page
	c.OnHTML("div.views-row", func(div *colly.HTMLElement) {
		children := div.DOM.Children()
		h3 := children.Find("div[aria-label]")

		header_components := strings.Split(h3.Text(), " - ")
		if len(header_components) > 1 {
			code := strings.TrimSpace(header_components[0])
			name := strings.TrimSpace(header_components[1])
			fmt.Printf("%s: %s\n", code, name)

			Courses[code] = CourseInfo{name: name}
		}

	})

	// Go to next page
	c.OnHTML("li.pager__item--next", func(next_page_button *colly.HTMLElement) {
		a := next_page_button.DOM.Find("a[href]")
		link, nextPageExists := a.Attr("href")
		if nextPageExists {
			c.Visit(next_page_button.Request.AbsoluteURL(link))
		}
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("Visiting %s\n", r.URL.String())
	})

	c.Visit("https://artsci.calendar.utoronto.ca/search-courses")
}
