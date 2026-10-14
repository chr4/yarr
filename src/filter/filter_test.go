package filter

import (
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

func TestCompile(t *testing.T) {
	for _, pattern := range []string{"", "  ", "x*", "a|", "("} {
		if _, err := Compile(pattern, false); err == nil {
			t.Errorf("expected %q to be rejected", pattern)
		}
	}
	re, err := Compile("sponsored", false)
	if err != nil || !re.MatchString("SPONSORED post") {
		t.Errorf("expected a case-insensitive match, got %v", err)
	}
	re, err = Compile("sponsored", true)
	if err != nil || re.MatchString("SPONSORED post") {
		t.Errorf("expected a case-sensitive match, got %v", err)
	}
}

func TestMatch(t *testing.T) {
	feed := int64(1)
	m := New([]model.Filter{
		{Id: 1, Pattern: "disabled", Enabled: false},
		{Id: 2, Pattern: "^\\[ad\\]", FeedId: &feed, Enabled: true},
		{Id: 3, Pattern: "giveaway", MatchContent: true, Enabled: true},
	})
	if !m.NeedsContent() {
		t.Error("expected the matcher to need content")
	}

	tests := []struct {
		item model.Item
		want int64
	}{
		{model.Item{FeedId: 1, Title: "disabled"}, 0},
		{model.Item{FeedId: 1, Title: "[Ad] buy now"}, 2},
		{model.Item{FeedId: 2, Title: "[Ad] buy now"}, 0},
		{model.Item{FeedId: 2, Title: "news", Content: "<p>give a <b>way</b></p>"}, 0},
		{model.Item{FeedId: 2, Title: "news", Content: "<p>a <b>Give</b>away</p>"}, 3},
	}
	for _, tt := range tests {
		have := m.Match(tt.item)
		if (have == nil && tt.want != 0) || (have != nil && *have != tt.want) {
			t.Errorf("%+v: expected filter %d, got %v", tt.item, tt.want, have)
		}
	}
}

func TestPreview(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	feed := db.CreateFeed(model.CreateFeedParams{Title: "feed", FeedLink: "http://example.com/feed.xml"})
	long := "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor "
	db.CreateItems([]model.Item{
		{GUID: "1", FeedId: feed.Id, Title: "Weekly deals", Date: time.Now().Add(-time.Hour)},
		{GUID: "2", FeedId: feed.Id, Title: "news", Content: "<p>" + long + "best deal ever " + long + "</p>", Date: time.Now()},
		{GUID: "3", FeedId: feed.Id, Title: "unrelated", Date: time.Now()},
	})

	count, list, err := Preview(db, model.FilterParams{Pattern: "deal", MatchContent: true}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || len(list) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", count, list)
	}
	// newest first
	if list[0].Field != "content" || list[0].Match != "deal" ||
		list[0].Before[:len("…")] != "…" || list[0].After[len(list[0].After)-len("…"):] != "…" {
		t.Errorf("invalid content snippet: %+v", list[0])
	}
	if list[1].Field != "title" || list[1].Before != "Weekly " || list[1].Match != "deal" || list[1].After != "s" {
		t.Errorf("invalid title match: %+v", list[1])
	}

	count, list, _ = Preview(db, model.FilterParams{Pattern: "deal"}, 1)
	if count != 1 || len(list) != 1 {
		t.Errorf("expected only the title match, got %d", count)
	}

	if _, _, err := Preview(db, model.FilterParams{Pattern: "("}, 10); err == nil {
		t.Error("expected an invalid pattern to fail")
	}
}
