package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/proxy"

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

type ProgramCode string

type Program struct {
	faculty string
	code    string
	name    string
}

type ProgramSubjectArea struct {
	name     string
	endpoint string
}

var (
	Courses             map[CourseCode]Course = make(map[CourseCode]Course)
	ProgramSubjectAreas []ProgramSubjectArea
	Programs            map[ProgramCode]Program = make(map[ProgramCode]Program)
	DB                  *sql.DB
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

	// go getAllCourses()
	getAllProgramSubjectAreas()
	getAllPrograms()

	// for _, programSubjectArea := range ProgramSubjectAreas {
	// 	fmt.Println(programSubjectArea)
	// }

	err = addCoursesToDatabase(Courses)
	if err != nil {
		log.Fatal(fmt.Errorf("couldn't add courses to database: %w", err))
	}
}

func getAllCourses() {

	c := colly.NewCollector(
		colly.AllowedDomains("artsci.calendar.utoronto.ca"),
	)

	// Rotate two socks5 proxies
	rp, err := proxy.RoundRobinProxySwitcher("socks5://127.0.0.1:1337", "socks5://127.0.0.1:1338")
	if err != nil {
		log.Fatal(err)
	}
	c.SetProxyFunc(rp)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
		RandomDelay: 5 * time.Second,
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

func getAllProgramSubjectAreas() {

	c := colly.NewCollector(
		colly.AllowedDomains("artsci.calendar.utoronto.ca"),
	)

	// Rotate two socks5 proxies
	// rp, err := proxy.RoundRobinProxySwitcher("socks5://127.0.0.1:1337", "socks5://127.0.0.1:1338")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// c.SetProxyFunc(rp)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       2 * time.Second,
		RandomDelay: 5 * time.Second,
	})

	// Get courses from each page
	c.OnHTML("td", func(td *colly.HTMLElement) {
		a := td.DOM.Find("a")
		programSubjectAreaName := a.Text()
		target, targetExists := a.Attr("href")

		if targetExists && strings.Contains(target, "/section/") {
			programSubjectArea := ProgramSubjectArea{name: programSubjectAreaName, endpoint: target}
			ProgramSubjectAreas = append(ProgramSubjectAreas, programSubjectArea)
		}

	})

	c.OnRequest(func(r *colly.Request) {
		fmt.Printf("Visiting %s\n", r.URL.String())
	})

	c.Visit("https://artsci.calendar.utoronto.ca/listing-program-subject-areas")
}

func getAllPrograms() {

	fmt.Println("Getting all programs...")

	for _, programSubjectArea := range ProgramSubjectAreas {

		c := colly.NewCollector(
			colly.AllowedDomains("artsci.calendar.utoronto.ca"),
		)

		// Rotate two socks5 proxies
		// rp, err := proxy.RoundRobinProxySwitcher("socks5://127.0.0.1:1337", "socks5://127.0.0.1:1338")
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// c.SetProxyFunc(rp)

		c.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			Parallelism: 1,
			Delay:       2 * time.Second,
			RandomDelay: 5 * time.Second,
		})

		c.OnHTML("h3", func(h3 *colly.HTMLElement) {
			aria_label_div := h3.DOM.Find("div[aria-label]")

			isValidProgram := false
			programStringsLowercase := []string{"specialist", "major", "minor", "certificate", "focus", "science program", "arts program"}
			for _, programStringLowercase := range programStringsLowercase {
				parenthesizedProgramStringLowercase := fmt.Sprintf("(%s)", programStringLowercase)
				if strings.Contains(strings.ToLower(aria_label_div.Text()), parenthesizedProgramStringLowercase) {
					isValidProgram = true
					break
				}
			}

			if isValidProgram {
				string_components := strings.Split(aria_label_div.Text(), " - ")
				if len(string_components) > 1 {
					code := strings.TrimSpace(string_components[1])
					name := strings.TrimSpace(string_components[0])

					programCodePrefixes := []string{"ASMIN", "ASMAJ", "ASSPE", "ASCER", "ASFOC"}
					for _, prefix := range programCodePrefixes {
						if code[0:len(prefix)] == prefix {
							fmt.Printf("%s: %s\n", code, name)
							Programs[ProgramCode(code)] = Program{code: code, name: name}
						}
					}

				}
			}

		})

		fmt.Println("Visiting ", fmt.Sprintf("https://artsci.calendar.utoronto.ca%s\n", programSubjectArea.endpoint))
		err := c.Visit(fmt.Sprintf("https://artsci.calendar.utoronto.ca%s", programSubjectArea.endpoint))
		if err != nil {
			fmt.Println(err)
			break
		}
	}

	fmt.Println("Got all programs.")
}
