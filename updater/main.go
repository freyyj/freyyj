package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/pkg/errors"
)

func main() {
	var readmeData READMEData
	var wg sync.WaitGroup

	//
	// Articles
	//
	wg.Add(1)
	go func() {
		var err error
		readmeData.Articles, err = getAtomFeedEntries(baseURL + "/articles.atom")
		if err != nil {
			fail(err)
		}

		wg.Done()
	}()

	wg.Wait()

	err := renderTemplateToStdout(&readmeData)
	if err != nil {
		fail(err)
	}
}

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Private
//
//
//
//////////////////////////////////////////////////////////////////////////////

const baseURL = "https://freyyj.org"

const (
	categorySymbolOther = "🗞️"

	categorySymbolBoyToGirl       = "💄"
	categorySymbolCulture         = "🎞️"
	categorySymbolPersonal        = "📓"
	categorySymbolSocialPlatforms = "🌏"
	categorySymbolTechnology      = "🖥️"

	categoryTermBoyToGirl       = "boy-to-girl"
	categoryTermCulture         = "culture"
	categoryTermPersonal        = "personal"
	categoryTermSocialPlatforms = "social-platforms"
	categoryTermTechnology      = "technology"
)

// A backoff schedule for HTTP requests.
var backoffSchedule = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	10 * time.Second,
}

// Helper functions made available during template rendering.
var templateFuncMap = template.FuncMap{
	"SymbolForCategories": symbolForCategories,
}

// Category is a category of an Atom entry.
type Category struct {
	Term string `xml:"term,attr"`
}

// Feed represents an Atom feed. Used for deserializing XML.
type Feed struct {
	XMLName xml.Name `xml:"feed"`

	Entries []*Entry `xml:"entry"`
	Title   string   `xml:"title"`
}

// Entry represents an entry in an Atom feed. Used for deserializing XML.
type Entry struct {
	Categories []*Category `xml:"category"`
	Link       struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Summary   string    `xml:"summary"`
	Title     string    `xml:"title"`
	Published time.Time `xml:"published"`
}

// READMEData is a struct containing all the information necessary to render a
// new version of `README.md`.
type READMEData struct {
	Articles []*Entry
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "Error during execution:\n%v\n", err)
	os.Exit(1)
}

func getAtomFeedEntries(url string) ([]*Entry, error) {
	resp, body, err := getURLDataWithRetries(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "Non-200 status code fetching URL '%s': %v",
			url, string(body))
	}

	var feed Feed

	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return nil, errors.Wrapf(err, "Error unmarshaling Atom feed XML")
	}

	return feed.Entries, nil
}

// Gets data at a URL. Connects and reads the entire response string, but
// notably does not check for problems with bad status codes.
func getURLData(url string) (*http.Response, []byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Error fetching URL '%s'", url)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Error reading response body from URL '%s'", url)
	}

	return resp, body, nil
}

func getURLDataWithRetries(url string) (*http.Response, []byte, error) {
	var body []byte
	var err error
	var resp *http.Response

	for _, backoff := range backoffSchedule {
		resp, body, err = getURLData(url)

		if err == nil {
			break
		}

		fmt.Fprintf(os.Stderr, "Request error: %+v\n", err)
		fmt.Fprintf(os.Stderr, "Retrying in %v\n", backoff)
		time.Sleep(backoff)
	}

	// All retries failed
	if err != nil {
		return nil, nil, err
	}

	return resp, body, nil
}

func renderTemplateToStdout(readmeData *READMEData) error {
	readmeTemplate := template.Must(
		template.New("").Funcs(templateFuncMap).ParseFiles("README.md.tmpl"),
	)

	err := readmeTemplate.ExecuteTemplate(os.Stdout, "README.md.tmpl", readmeData)
	if err != nil {
		return errors.Wrap(err, "Error rendering README.md template")
	}

	return nil
}

// Returns a symbol representing the category of an entry. Part of its job is
// to prioritize which categories should be preferred over others.
func symbolForCategories(categories []*Category) string {
	categoryMap := make(map[string]struct{})

	for _, category := range categories {
		categoryMap[category.Term] = struct{}{}
	}

	//
	// Basicaly ordered according to interestingness, and to give boy-to-girl
	// stuff more prominence given ambiguity in multiple categories as it's the
	// type of content most people on GitHub aren't likely to care about (so
	// they can choose not to click on it).
	//

	if _, ok := categoryMap[categoryTermTechnology]; ok {
		return categorySymbolTechnology
	}

	if _, ok := categoryMap[categoryTermCulture]; ok {
		return categorySymbolCulture
	}

	if _, ok := categoryMap[categoryTermBoyToGirl]; ok {
		return categorySymbolBoyToGirl
	}

	if _, ok := categoryMap[categoryTermCulture]; ok {
		return categorySymbolCulture
	}

	if _, ok := categoryMap[categoryTermPersonal]; ok {
		return categorySymbolPersonal
	}

	return categorySymbolOther
}
