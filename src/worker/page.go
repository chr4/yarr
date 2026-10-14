package worker

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"

	"github.com/nkanaev/yarr/src/filter"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	// pages are cut off after this many bytes, before and after reducing them
	pageMaxSize = 2 << 20
	pageMaxKept = 512 << 10

	// only the newest items of a feed have their page fetched afterwards
	PageWindow = 100

	// pages fetched at once for a feed, and stored together
	pageFetchers  = 2
	pageBatchSize = 10
)

// FetchPage downloads the article page of an item and reduces it to what
// filters match against: the page source without scripts, styles and
// comments, with the text unescaped and whitespace collapsed.
func FetchPage(url string) (string, error) {
	if url == "" {
		return "", errors.New("item has no link")
	}
	return fetchPage(url, IsInternalURL)
}

func fetchPage(url string, internal func(url string) bool) (string, error) {
	if internal(url) {
		return "", fmt.Errorf("refusing to fetch internal address %s", url)
	}
	// news sites often redirect until the cookie they set (e.g. for consent)
	// comes back, so each page gets cookies of its own, kept for its redirects
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", err
	}
	httpClient := *client.httpClient
	httpClient.Jar = jar
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if internal(req.URL.String()) {
			return fmt.Errorf("refusing to follow redirect to internal address %s", req.URL)
		}
		return nil
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", client.userAgent)
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	r, err := charset.NewReader(io.LimitReader(res.Body, pageMaxSize), res.Header.Get("Content-Type"))
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	// looked for before reducing the page, which drops the scripts
	if isChallenge(res, string(body)) {
		return "", ErrBlocked
	}
	// error pages are kept as well: a paywall may well answer with 402 or 403
	return ReducePage(strings.NewReader(string(body))), nil
}

// ErrBlocked tells that the site answered with a captcha or a similar
// challenge instead of the page.
var ErrBlocked = errors.New("blocked by a captcha")

// markers of the pages bot protection services answer with instead of the
// page asked for. A heuristic: unknown services go unnoticed.
var challengeMarkers = []string{
	"/cdn-cgi/challenge-platform/", // Cloudflare
	"captcha-delivery.com",         // DataDome
	"px-captcha",                   // PerimeterX
}

// captcha widgets, which also show on article pages, e.g. for comments,
// so that they only count on error pages
var captchaMarkers = []string{
	"google.com/recaptcha",
	"hcaptcha.com",
	"challenges.cloudflare.com/turnstile",
}

func isChallenge(res *http.Response, body string) bool {
	if res.Header.Get("Cf-Mitigated") == "challenge" {
		return true
	}
	for _, marker := range challengeMarkers {
		if strings.Contains(body, marker) {
			return true
		}
	}
	if res.StatusCode < 400 {
		return false
	}
	for _, marker := range captchaMarkers {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

// skipped elements, with their content
var skippedTags = map[string]bool{
	"script":   true,
	"style":    true,
	"svg":      true,
	"template": true,
}

// ReducePage keeps the tags, attributes and text of a page, leaving out
// scripts (except structured data, which tells about paywalls), styles,
// inline images and comments.
func ReducePage(r io.Reader) string {
	var b strings.Builder
	z := html.NewTokenizer(r)
	skipping := ""
	for b.Len() < pageMaxKept {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if skipping != "" {
			if tt == html.EndTagToken {
				if name, _ := z.TagName(); string(name) == skipping {
					skipping = ""
				}
			}
			continue
		}
		switch tt {
		case html.TextToken:
			text := strings.Join(strings.Fields(html.UnescapeString(string(z.Text()))), " ")
			if text != "" {
				b.WriteString(text)
				b.WriteByte(' ')
			}
		case html.StartTagToken:
			raw := string(z.Raw())
			name, _ := z.TagName()
			if skippedTags[string(name)] && !isStructuredData(z) {
				skipping = string(name)
				continue
			}
			b.WriteString(strings.Join(strings.Fields(raw), " "))
		case html.EndTagToken, html.SelfClosingTagToken:
			b.WriteString(strings.Join(strings.Fields(string(z.Raw())), " "))
		}
	}
	return strings.TrimSpace(b.String())
}

// isStructuredData reports whether the current script tag holds JSON-LD.
// Must be called after Raw and TagName, as it reads the attributes.
func isStructuredData(z *html.Tokenizer) bool {
	for {
		key, val, more := z.TagAttr()
		if string(key) == "type" && strings.Contains(strings.ToLower(string(val)), "ld+json") {
			return true
		}
		if !more {
			return false
		}
	}
}

// fetchNewPages fetches the article pages of a feed's new items, if any
// filter checks them, so that they are hidden before they are stored.
func fetchNewPages(db storage.Storage, feed model.Feed, items []model.Item) {
	if len(items) == 0 || !filter.New(db.ListFilters()).NeedsPage(feed.Id) {
		return
	}
	known := db.ListItemGUIDs(feed.Id)
	for i := range items {
		if known[items[i].GUID] {
			continue
		}
		page := fetchPageOrEmpty(items[i].Link)
		items[i].Page = &page.Content
		items[i].PageBlocked = page.Blocked
	}
}

// fetchPageOrEmpty returns an empty page when fetching fails, which is
// stored like any other, so that a broken link is not fetched again and again.
// Pages behind a captcha are marked so, to tell why they cannot be checked.
func fetchPageOrEmpty(url string) model.ItemPage {
	page, err := FetchPage(url)
	if err != nil {
		log.Printf("failed to fetch page %s: %s", url, err)
	}
	return model.ItemPage{Content: page, Blocked: errors.Is(err, ErrBlocked)}
}

// feeds whose pages are being fetched, with the number of pages left
var pageJobs = struct {
	sync.Mutex
	left map[int64]int
}{left: make(map[int64]int)}

// PagesLeft reports how many pages of a feed are left to fetch, and whether
// they are being fetched.
func PagesLeft(feedId int64) (int, bool) {
	pageJobs.Lock()
	defer pageJobs.Unlock()
	left, ok := pageJobs.left[feedId]
	return left, ok
}

// FetchMissingPages fetches, in the background, the pages missing among the
// newest items of a feed, and re-applies the filters as they arrive.
// It reports whether the pages are being fetched.
func FetchMissingPages(db storage.Storage, feedId int64) bool {
	busy, started := fetchPages(db, feedId, true)
	return busy || started
}

// RefetchPages fetches the pages of the newest items of a feed again, in the
// background, replacing the stored ones as they arrive so that items stay
// hidden meanwhile. It reports false if the pages are being fetched already.
func RefetchPages(db storage.Storage, feedId int64) bool {
	busy, _ := fetchPages(db, feedId, false)
	return !busy
}

// fetchPages reports whether the pages of the feed were being fetched
// already, and else whether it started fetching them.
func fetchPages(db storage.Storage, feedId int64, onlyMissing bool) (busy, started bool) {
	pageJobs.Lock()
	defer pageJobs.Unlock()
	if _, ok := pageJobs.left[feedId]; ok {
		return true, false
	}
	items := db.ListPageItems(feedId, PageWindow, onlyMissing)
	if len(items) == 0 {
		return false, false
	}
	pageJobs.left[feedId] = len(items)

	go func() {
		defer func() {
			pageJobs.Lock()
			delete(pageJobs.left, feedId)
			pageJobs.Unlock()
		}()
		log.Printf("fetching %d pages of feed %d", len(items), feedId)

		for start := 0; start < len(items); start += pageBatchSize {
			batch := items[start:min(start+pageBatchSize, len(items))]
			pages := make([]model.ItemPage, len(batch))
			var wg sync.WaitGroup
			sem := make(chan struct{}, pageFetchers)
			for i, item := range batch {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					pages[i] = fetchPageOrEmpty(item.Link)
					<-sem
				}()
			}
			wg.Wait()

			stored := make(map[int64]model.ItemPage, len(batch))
			for i, item := range batch {
				stored[item.Id] = pages[i]
			}
			if !db.SetItemPages(stored) {
				return
			}
			if err := filter.Refilter(db); err != nil {
				log.Print(err)
			}
			pageJobs.Lock()
			pageJobs.left[feedId] -= len(batch)
			pageJobs.Unlock()
		}
	}()
	return false, true
}
