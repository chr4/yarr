package tests

import (
	"slices"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/filter"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

func listedGUIDs(db storage.Storage, f model.ItemFilter) []string {
	guids := make([]string, 0)
	for _, item := range db.ListItems(f, 100, false, false) {
		guids = append(guids, item.GUID)
	}
	slices.Sort(guids)
	return guids
}

func unreadStat(db storage.Storage, feedId int64) int64 {
	for _, stat := range db.FeedStats() {
		if stat.FeedId == feedId {
			return stat.UnreadCount
		}
	}
	return -1
}

func TestFilterCRUD(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		feed := db.CreateFeed(model.CreateFeedParams{Title: "feed", FeedLink: "http://example.com/feed.xml"})

		global := db.CreateFilter(model.FilterParams{Pattern: "sponsored", Enabled: true})
		scoped := db.CreateFilter(model.FilterParams{
			Pattern: "^\\[ad\\]", MatchContent: true, FeedId: &feed.Id, Enabled: true,
		})
		if global == nil || scoped == nil {
			t.Fatal("failed to create filters")
		}

		if !db.UpdateFilter(global.Id, model.FilterParams{Pattern: "promo", CaseSensitive: true}) {
			t.Fatal("failed to update filter")
		}

		have := db.ListFilters()
		want := []model.Filter{
			{Id: global.Id, Pattern: "promo", CaseSensitive: true},
			{Id: scoped.Id, Pattern: "^\\[ad\\]", MatchContent: true, FeedId: &feed.Id, Enabled: true},
		}
		if len(have) != 2 || have[0] != want[0] || have[1].Pattern != want[1].Pattern ||
			*have[1].FeedId != feed.Id || !have[1].MatchContent || !have[1].Enabled {
			t.Fatalf("invalid filters\nhave: %+v\nwant: %+v", have, want)
		}

		// filters for a feed go away with the feed
		db.DeleteFeed(feed.Id)
		if have := db.ListFilters(); len(have) != 1 || have[0].Id != global.Id {
			t.Fatalf("expected only the global filter, got %+v", have)
		}

		db.DeleteFilter(global.Id)
		if have := db.ListFilters(); len(have) != 0 {
			t.Fatalf("expected no filters, got %+v", have)
		}
	})
}

func TestFilteredItems(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		scope := testItemsSetup(db)
		all := listedGUIDs(db, model.ItemFilter{})

		// item111 is unread, item112 read, item113 starred
		f := db.CreateFilter(model.FilterParams{Pattern: "TITLE11", Enabled: true})
		if err := filter.Refilter(db); err != nil {
			t.Fatal(err)
		}

		have := listedGUIDs(db, model.ItemFilter{})
		want := slices.DeleteFunc(slices.Clone(all), func(g string) bool {
			return g == "item111" || g == "item112"
		})
		if !slices.Equal(have, want) {
			t.Fatalf("starred and unmatched items must be listed\nhave: %v\nwant: %v", have, want)
		}
		if n := unreadStat(db, scope.feed11.Id); n != 0 {
			t.Fatalf("hidden items must not count as unread, got %d", n)
		}
		if have := db.ListFilters(); have[0].Hidden != 2 {
			t.Fatalf("expected 2 hidden items, got %d", have[0].Hidden)
		}

		// new items are filtered on arrival
		items := []model.Item{
			{GUID: "new1", FeedId: scope.feed01.Id, Title: "Title11 again", Date: time.Now()},
			{GUID: "new2", FeedId: scope.feed01.Id, Title: "other", Date: time.Now()},
		}
		filter.ApplyStored(db, items)
		db.CreateItems(items)
		have = listedGUIDs(db, model.ItemFilter{FeedID: &scope.feed01.Id})
		if !slices.Contains(have, "new2") || slices.Contains(have, "new1") {
			t.Fatalf("expected new1 to be hidden, got %v", have)
		}

		// disabling the filter brings the items back as they were
		db.UpdateFilter(f.Id, model.FilterParams{Pattern: "TITLE11", Enabled: false})
		if err := filter.Refilter(db); err != nil {
			t.Fatal(err)
		}
		if have := listedGUIDs(db, model.ItemFilter{FeedID: &scope.feed11.Id}); len(have) != 3 {
			t.Fatalf("expected all items of feed11, got %v", have)
		}
		if n := unreadStat(db, scope.feed11.Id); n != 1 {
			t.Fatalf("expected item111 to be unread again, got %d unread", n)
		}
	})
}

func TestItemPages(t *testing.T) {
	dbtest(t, func(t *testing.T, db storage.Storage) {
		feed := db.CreateFeed(model.CreateFeedParams{Title: "feed", FeedLink: "http://example.com/feed.xml"})
		paywalled := `<script type="application/ld+json">{"isAccessibleForFree": false}</script>`
		db.CreateItems([]model.Item{
			{GUID: "paywalled", FeedId: feed.Id, Title: "one", Link: "http://example.com/1", Page: &paywalled, Date: time.Now().Add(-2 * time.Hour)},
			{GUID: "free", FeedId: feed.Id, Title: "two", Link: "http://example.com/2", Date: time.Now().Add(-time.Hour)},
			{GUID: "newest", FeedId: feed.Id, Title: "three", Link: "http://example.com/3", Date: time.Now()},
		})

		if have := db.ListItemGUIDs(feed.Id); len(have) != 3 || !have["free"] {
			t.Fatalf("invalid guids: %v", have)
		}

		// only the newest items are looked at
		missing := db.ListPageItems(feed.Id, 2, true)
		if len(missing) != 2 || missing[0].Link != "http://example.com/3" || missing[1].Link != "http://example.com/2" {
			t.Fatalf("expected the two newest items without page, got %+v", missing)
		}
		if missing := db.ListPageItems(feed.Id, 1, true); len(missing) != 1 {
			t.Fatalf("expected one item without page, got %+v", missing)
		}

		// arriving again leaves the stored page alone
		db.CreateItems([]model.Item{{GUID: "paywalled", FeedId: feed.Id, Title: "one", Date: time.Now()}})

		if !db.SetItemPages(map[int64]model.ItemPage{
			missing[0].Id: {Content: "<p>free</p>"},
			missing[1].Id: {Blocked: true},
		}) {
			t.Fatal("failed to store pages")
		}
		if missing := db.ListPageItems(feed.Id, 10, true); len(missing) != 0 {
			t.Fatalf("expected no missing pages, got %+v", missing)
		}
		if all := db.ListPageItems(feed.Id, 10, false); len(all) != 3 {
			t.Fatalf("expected all items to be listed for fetching again, got %+v", all)
		}

		pages := make(map[string]*string)
		db.ForEachItem(false, true, func(item model.Item) { pages[item.Title] = item.Page })
		if pages["one"] == nil || *pages["one"] != paywalled || pages["two"] == nil || *pages["two"] != "" {
			t.Fatalf("invalid pages: %v", pages)
		}
		db.ForEachItem(false, false, func(item model.Item) {
			if item.Page != nil {
				t.Fatalf("expected no page without asking, got %q", *item.Page)
			}
		})

		f := db.CreateFilter(model.FilterParams{
			Pattern: `"isAccessibleForFree":\s*false`, MatchPage: true, FeedId: &feed.Id, Enabled: true,
		})
		if !db.ListFilters()[0].MatchPage {
			t.Fatal("expected the filter to check the page")
		}
		if n := db.CountBlockedPages(feed.Id); n != 1 {
			t.Fatalf("expected one blocked page, got %d", n)
		}
		if n := db.ListFilters()[0].Blocked; n != 1 {
			t.Fatalf("expected the filter to report one blocked page, got %d", n)
		}
		if err := filter.Refilter(db); err != nil {
			t.Fatal(err)
		}
		if have := listedGUIDs(db, model.ItemFilter{}); !slices.Equal(have, []string{"free", "newest"}) {
			t.Fatalf("expected the paywalled item to be hidden, got %v", have)
		}

		// pages go away with their items
		db.DeleteFilter(f.Id)
		db.DeleteFeed(feed.Id)
		count := 0
		db.ForEachItem(false, true, func(model.Item) { count++ })
		if count != 0 {
			t.Fatalf("expected no items left, got %d", count)
		}
	})
}
