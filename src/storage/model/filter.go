package model

// Filter hides items whose title (and optionally content) matches a regular
// expression. Hidden items keep their status, so removing a filter brings
// them back as they were.
type Filter struct {
	Id            int64  `json:"id"`
	Pattern       string `json:"pattern"`
	MatchContent  bool   `json:"match_content"`
	CaseSensitive bool   `json:"case_sensitive"`
	FeedId        *int64 `json:"feed_id"`
	Enabled       bool   `json:"enabled"`

	// number of items currently hidden by this filter
	Hidden int64 `json:"hidden"`
}

type FilterParams struct {
	Pattern       string `json:"pattern"`
	MatchContent  bool   `json:"match_content"`
	CaseSensitive bool   `json:"case_sensitive"`
	FeedId        *int64 `json:"feed_id"`
	Enabled       bool   `json:"enabled"`
}
