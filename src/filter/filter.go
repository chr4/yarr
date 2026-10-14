// Package filter hides items whose title, content or article page matches
// user-defined regular expressions.
package filter

import (
	"cmp"
	"errors"
	"log"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nkanaev/yarr/src/content/htmlutil"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

var (
	ErrEmptyPattern  = errors.New("pattern is empty")
	ErrMatchesAll    = errors.New("pattern matches every article")
	ErrPageNeedsFeed = errors.New("checking the article page needs a single feed")
)

// Compile returns the regular expression of a filter pattern.
// Patterns are case-insensitive unless asked otherwise.
func Compile(pattern string, caseSensitive bool) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, ErrEmptyPattern
	}
	// checked as given first, so that errors quote the pattern the user wrote
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	if !caseSensitive {
		re = regexp.MustCompile("(?i)" + pattern)
	}
	if re.MatchString("") {
		return nil, ErrMatchesAll
	}
	return re, nil
}

type rule struct {
	id      int64
	feedId  *int64
	re      *regexp.Regexp
	content bool
	page    bool
}

// itemText extracts the text of an item's content at most once.
type itemText struct {
	item *model.Item
	text *string
}

func (t *itemText) content() string {
	if t.text == nil {
		text := htmlutil.ExtractText(t.item.Content)
		t.text = &text
	}
	return *t.text
}

// find returns the field and text the rule matched in, and the match location.
func (r *rule) find(item *model.Item, text *itemText) (string, string, []int) {
	if r.feedId != nil && *r.feedId != item.FeedId {
		return "", "", nil
	}
	if loc := r.re.FindStringIndex(item.Title); loc != nil {
		return "title", item.Title, loc
	}
	if r.content {
		content := text.content()
		if loc := r.re.FindStringIndex(content); loc != nil {
			return "content", content, loc
		}
	}
	if r.page && item.Page != nil {
		if loc := r.re.FindStringIndex(*item.Page); loc != nil {
			return "page", *item.Page, loc
		}
	}
	return "", "", nil
}

// Check returns why a filter cannot be used, if it cannot.
func Check(params model.FilterParams) error {
	_, err := newRule(0, params)
	return err
}

func newRule(id int64, params model.FilterParams) (*rule, error) {
	// pages are fetched only for the feeds that need them
	if params.MatchPage && params.FeedId == nil {
		return nil, ErrPageNeedsFeed
	}
	re, err := Compile(params.Pattern, params.CaseSensitive)
	if err != nil {
		return nil, err
	}
	return &rule{id: id, feedId: params.FeedId, re: re, content: params.MatchContent, page: params.MatchPage}, nil
}

// Matcher decides which of the enabled filters hides an item.
type Matcher struct {
	rules []*rule
}

func New(filters []model.Filter) *Matcher {
	m := &Matcher{}
	for _, f := range filters {
		if !f.Enabled {
			continue
		}
		r, err := newRule(f.Id, model.FilterParams{
			Pattern:       f.Pattern,
			MatchContent:  f.MatchContent,
			MatchPage:     f.MatchPage,
			CaseSensitive: f.CaseSensitive,
			FeedId:        f.FeedId,
		})
		if err != nil {
			log.Printf("skipping filter %d: %s", f.Id, err)
			continue
		}
		m.rules = append(m.rules, r)
	}
	return m
}

// NeedsContent reports whether any filter looks at the item content.
func (m *Matcher) NeedsContent() bool {
	return slices.ContainsFunc(m.rules, func(r *rule) bool { return r.content })
}

// NeedsPage reports whether any filter looks at the article page of
// a feed's items.
func (m *Matcher) NeedsPage(feedId int64) bool {
	return slices.ContainsFunc(m.rules, func(r *rule) bool {
		return r.page && r.feedId != nil && *r.feedId == feedId
	})
}

// PageFeeds returns the feeds whose items have their article page checked.
func (m *Matcher) PageFeeds() []int64 {
	feeds := make([]int64, 0)
	for _, r := range m.rules {
		if r.page && !slices.Contains(feeds, *r.feedId) {
			feeds = append(feeds, *r.feedId)
		}
	}
	return feeds
}

// Match returns the id of the first filter hiding the item, or nil.
func (m *Matcher) Match(item model.Item) *int64 {
	text := &itemText{item: &item}
	for _, r := range m.rules {
		if field, _, _ := r.find(&item, text); field != "" {
			return &r.id
		}
	}
	return nil
}

// Apply marks the items hidden by a filter.
func (m *Matcher) Apply(items []model.Item) {
	for i := range items {
		items[i].FilterId = m.Match(items[i])
	}
}

// ApplyStored marks the items hidden by the filters in the database.
func ApplyStored(db storage.Storage, items []model.Item) {
	New(db.ListFilters()).Apply(items)
}

// Refilter re-evaluates which filter hides each stored item.
// Called whenever the filters change.
func Refilter(db storage.Storage) error {
	m := New(db.ListFilters())
	changes := make(map[int64]*int64)
	err := db.ForEachItem(m.NeedsContent(), len(m.PageFeeds()) > 0, func(item model.Item) {
		filterId := m.Match(item)
		old := item.FilterId
		if (old == nil) != (filterId == nil) || (old != nil && *old != *filterId) {
			changes[item.Id] = filterId
		}
	})
	if err != nil {
		return err
	}
	if len(changes) > 0 && !db.SetItemFilters(changes) {
		return errors.New("failed to update filtered items")
	}
	return nil
}

// PreviewMatch is a stored item matched by a filter, with the matched text
// cut out of the title or a snippet of the content or page.
type PreviewMatch struct {
	Id     int64            `json:"id"`
	FeedId int64            `json:"feed_id"`
	Title  string           `json:"title"`
	Date   time.Time        `json:"date"`
	Status model.ItemStatus `json:"status"`
	Field  string           `json:"field"`
	Before string           `json:"before"`
	Match  string           `json:"match"`
	After  string           `json:"after"`
}

const (
	snippetBefore = 40
	snippetAfter  = 80
)

// Preview returns how many stored items a filter would hide, and the newest
// of them up to the limit.
func Preview(db storage.Storage, params model.FilterParams, limit int) (int, []PreviewMatch, error) {
	r, err := newRule(0, params)
	if err != nil {
		return 0, nil, err
	}

	matches := make([]PreviewMatch, 0)
	err = db.ForEachItem(r.content, r.page, func(item model.Item) {
		field, text, loc := r.find(&item, &itemText{item: &item})
		if field == "" {
			return
		}
		match := PreviewMatch{
			Id:     item.Id,
			FeedId: item.FeedId,
			Title:  item.Title,
			Date:   item.Date,
			Status: item.Status,
			Field:  field,
			Before: text[:loc[0]],
			Match:  text[loc[0]:loc[1]],
			After:  text[loc[1]:],
		}
		if field != "title" {
			match.Before = cutLeft(match.Before, snippetBefore)
			match.After = cutRight(match.After, snippetAfter)
		}
		matches = append(matches, match)
	})
	if err != nil {
		return 0, nil, err
	}

	slices.SortFunc(matches, func(a, b PreviewMatch) int {
		return cmp.Or(b.Date.Compare(a.Date), cmp.Compare(b.Id, a.Id))
	})
	return len(matches), matches[:min(limit, len(matches))], nil
}

// cutLeft keeps the last n runes of s.
func cutLeft(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return "…" + strings.TrimLeft(string(runes[len(runes)-n:]), " ")
}

// cutRight keeps the first n runes of s.
func cutRight(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return strings.TrimRight(string(runes[:n]), " ") + "…"
}
