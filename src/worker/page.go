package worker

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/nkanaev/yarr/src/filter"
	"github.com/nkanaev/yarr/src/server"
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
func (w *Worker) FetchPage(url string) (string, error) {
	if url == "" {
		return "", errors.New("item has no link")
	}
	if server.IsInternalURL(url) {
		return "", fmt.Errorf("refusing to fetch internal address %s", url)
	}
	res, err := w.client.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// error pages are kept as well: a paywall may well answer with 402 or 403
	r, err := charset.NewReader(io.LimitReader(res.Body, pageMaxSize), res.Header.Get("Content-Type"))
	if err != nil {
		return "", err
	}
	return ReducePage(r), nil
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
func (w *Worker) fetchNewPages(db storage.Storage, feed model.Feed, items []model.Item) {
	if len(items) == 0 || !filter.New(db.ListFilters()).NeedsPage(feed.Id) {
		return
	}
	known := db.ListItemGUIDs(feed.Id)
	for i := range items {
		if known[items[i].GUID] {
			continue
		}
		page := w.fetchPageOrEmpty(items[i].Link)
		items[i].Page = &page
	}
}

// fetchPageOrEmpty returns an empty page when fetching fails, which is
// stored like any other, so that a broken link is not fetched again and again.
func (w *Worker) fetchPageOrEmpty(url string) string {
	page, err := w.FetchPage(url)
	if err != nil {
		log.Printf("failed to fetch page %s: %s", url, err)
	}
	return page
}

var pageJobs = struct {
	sync.Mutex
	feeds map[int64]bool
}{feeds: make(map[int64]bool)}

// FetchingPages reports whether the pages of a feed are being fetched.
func FetchingPages(feedId int64) bool {
	pageJobs.Lock()
	defer pageJobs.Unlock()
	return pageJobs.feeds[feedId]
}

var _ server.PageFetcher = (*Worker)(nil)

// MissingPages reports how many of the newest items of a feed have no page.
func (w *Worker) MissingPages(db storage.Storage, feedId int64) int {
	return len(db.ListItemsWithoutPage(feedId, PageWindow))
}

// FetchMissingPages fetches, in the background, the pages missing among the
// newest items of a feed, and re-applies the filters as they arrive.
// It reports whether there was anything to fetch.
func (w *Worker) FetchMissingPages(db storage.Storage, feedId int64) bool {
	pageJobs.Lock()
	defer pageJobs.Unlock()
	if pageJobs.feeds[feedId] {
		return true
	}
	items := db.ListItemsWithoutPage(feedId, PageWindow)
	if len(items) == 0 {
		return false
	}
	pageJobs.feeds[feedId] = true

	go func() {
		defer func() {
			pageJobs.Lock()
			delete(pageJobs.feeds, feedId)
			pageJobs.Unlock()
		}()
		log.Printf("fetching %d pages of feed %d", len(items), feedId)

		for start := 0; start < len(items); start += pageBatchSize {
			batch := items[start:min(start+pageBatchSize, len(items))]
			pages := make([]string, len(batch))
			var wg sync.WaitGroup
			sem := make(chan struct{}, pageFetchers)
			for i, item := range batch {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					pages[i] = w.fetchPageOrEmpty(item.Link)
					<-sem
				}()
			}
			wg.Wait()

			stored := make(map[int64]string, len(batch))
			for i, item := range batch {
				stored[item.Id] = pages[i]
			}
			if !db.SetItemPages(stored) {
				return
			}
			if err := filter.Refilter(db); err != nil {
				log.Print(err)
			}
		}
	}()
	return true
}
