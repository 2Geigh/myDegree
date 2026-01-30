package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly"

	"github.com/joho/godotenv"

	_ "github.com/go-sql-driver/mysql"
)

type CourseCode string

type Course struct {
	code        string
	faculty     string
	name        string
	description string
	prereqs     []CourseCode
}

type Program struct {
	faculty string
	code    string
	name    string
}

var (
	Courses map[CourseCode]Course = make(map[CourseCode]Course)
	DB      *sql.DB
)

func main() {
	godotenv.Load("../../.env")
	err := initializeDB(
		os.Getenv("MySQL_USERNAME"),
		os.Getenv("MySQL_PASSWORD"),
		os.Getenv("MySQL_DB_NAME"),
	)
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't initialize database: %w", err))
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go getAllCourses(&wg)
	wg.Add(1)
	go getAllPrograms(&wg)
	wg.Wait()

	err = addCoursesToDatabase(Courses)
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't add courses to database: %w", err))
	}
}

func getAllCourses(wg *sync.WaitGroup) {
	defer wg.Done()

	c := colly.NewCollector(
		colly.AllowedDomains("artsci.calendar.utoronto.ca"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	// Get courses from each page
	c.OnHTML("div.views-row", func(div *colly.HTMLElement) {
		children := div.DOM.Children()
		h3 := children.Find("div[aria-label]")

		header_components := strings.Split(h3.Text(), " - ")
		if len(header_components) > 1 {
			code := strings.TrimSpace(header_components[0])
			name := strings.TrimSpace(header_components[1])
			fmt.Printf("%s: %s\n", code, name)

			Courses[CourseCode(code)] = Course{code: code, name: name}
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

func addCoursesToDatabase(courses map[CourseCode]Course) error {
	var (
		totalRowsAffected int = 0
		err               error
	)

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("couldn't begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, course := range courses {
		stmt, err := tx.Prepare(`
		INSERT INTO Courses (code, name) VALUES (?, ?);
	`)
		if err != nil {
			return fmt.Errorf("couldn't prepare statement: %w", err)
		}

		result, err := stmt.Exec(course.code, course.name)
		if err != nil {
			return fmt.Errorf("couldn't execute statement: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("couldn't get rows affected: %w", err)
		}

		totalRowsAffected += int(rowsAffected)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("couldn't commit transaction: %w", err)
	}

	return err
}

func initializeDB(user string, password string, dbName string) error {
	log.Println("Initializing database...")
	var (
		err error
	)

	DB, err = sql.Open("mysql", fmt.Sprintf("%s:%s@/%s", user, password, dbName))
	if err != nil {
		return fmt.Errorf("couldn't open database connection: %w", err)
	}

	DB.SetConnMaxLifetime(time.Minute * 3)
	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(10)

	log.Println("Database initialized.")

	return err
}

func getAllPrograms(wg *sync.WaitGroup) {
	defer wg.Done()

	c := colly.NewCollector(
		colly.AllowedDomains("artsci.calendar.utoronto.ca"),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	// Get courses from each page
	c.OnHTML("article", func(article *colly.HTMLElement) {
		children := article.DOM.Children()
		w3_row := children.Find("div.w3-row")

		fmt.Println(w3_row.Text())
	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("Visiting %s\n", r.URL.String())
	})

	c.Visit("https://artsci.calendar.utoronto.ca/listing-program-subject-areas")
}
