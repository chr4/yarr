package model

// Filter hides items whose title (and optionally content or article page),
// or else link, matches a regular expression. Hidden items keep their status, so removing
// a filter brings them back as they were.
type Filter struct {
	Id            int64  `json:"id"`
	Pattern       string `json:"pattern"`
	MatchContent  bool   `json:"match_content"`
	MatchPage     bool   `json:"match_page"`
	MatchLink     bool   `json:"match_link"` // instead of the title
	CaseSensitive bool   `json:"case_sensitive"`
	FeedId        *int64 `json:"feed_id"`
	Enabled       bool   `json:"enabled"`

	// number of items currently hidden by this filter
	Hidden int64 `json:"hidden"`
	// number of article pages checked by this filter that are behind a captcha
	Blocked int64 `json:"blocked"`
}

type FilterParams struct {
	Pattern       string `json:"pattern"`
	MatchContent  bool   `json:"match_content"`
	MatchPage     bool   `json:"match_page"`
	MatchLink     bool   `json:"match_link"` // instead of the title
	CaseSensitive bool   `json:"case_sensitive"`
	FeedId        *int64 `json:"feed_id"`
	Enabled       bool   `json:"enabled"`
}

// ItemPage is the article page of an item, as fetched for filters.
type ItemPage struct {
	Content string
	// the site answered with a captcha or a similar challenge
	// instead of the page, leaving the content empty
	Blocked bool
}
