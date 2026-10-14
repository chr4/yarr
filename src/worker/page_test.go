package worker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nkanaev/yarr/src/client"
	"github.com/nkanaev/yarr/src/storage"
	"github.com/nkanaev/yarr/src/storage/model"
)

func TestReducePage(t *testing.T) {
	page := `<!doctype html>
<html><head>
	<style>.paywall { display: none }</style>
	<script>var paywall = true;</script>
	<script type="application/ld+json">{"isAccessibleForFree": false}</script>
</head>
<body>
	<!-- paywall comment -->
	<svg><path d="M0 0"/></svg>
	<div   class="paywall"
	       data-x="1">Jetzt&nbsp;weiterlesen   mit <b>Abo</b></div>
	<img src="a.png" />
</body></html>`
	want := `<html><head><script type="application/ld+json">{"isAccessibleForFree": false} </script></head>` +
		`<body><div class="paywall" data-x="1">Jetzt weiterlesen mit <b>Abo </b></div><img src="a.png" /></body></html>`
	if have := ReducePage(strings.NewReader(page)); have != want {
		t.Errorf("invalid page\nhave: %s\nwant: %s", have, want)
	}
}

func TestFetchPageInternal(t *testing.T) {
	w := NewWorker(nil, client.NewBuilder().Build())
	if _, err := w.FetchPage("http://127.0.0.1/secret"); err == nil {
		t.Error("expected internal addresses to be refused")
	}
	if _, err := w.FetchPage(""); err == nil {
		t.Error("expected an empty link to be refused")
	}
}

func TestFetchNewPages(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	w := NewWorker(db, client.NewBuilder().Build())
	feed := db.CreateFeed(model.CreateFeedParams{Title: "feed", FeedLink: "http://example.com/feed.xml"})
	other := db.CreateFeed(model.CreateFeedParams{Title: "other", FeedLink: "http://example.com/other.xml"})
	db.CreateItems([]model.Item{{GUID: "known", FeedId: feed.Id, Title: "known", Date: time.Now()}})

	// links that cannot be fetched, which leaves the pages empty
	items := []model.Item{
		{GUID: "known", FeedId: feed.Id, Link: "http://127.0.0.1/known"},
		{GUID: "new", FeedId: feed.Id, Link: "http://127.0.0.1/new"},
	}
	w.fetchNewPages(db, *feed, items)
	if items[0].Page != nil {
		t.Error("expected no page without a page filter")
	}

	db.CreateFilter(model.FilterParams{Pattern: "paywall", MatchPage: true, FeedId: &feed.Id, Enabled: true})
	w.fetchNewPages(db, *feed, items)
	if items[0].Page != nil || items[1].Page == nil || *items[1].Page != "" {
		t.Errorf("expected only the new item to have its page fetched, got %v, %v", items[0].Page, items[1].Page)
	}

	otherItems := []model.Item{{GUID: "new", FeedId: other.Id, Link: "http://127.0.0.1/new"}}
	w.fetchNewPages(db, *other, otherItems)
	if otherItems[0].Page != nil {
		t.Error("expected no page for a feed without page filter")
	}
}

func TestFetchPageCookieRedirect(t *testing.T) {
	wrk := NewWorker(nil, client.NewBuilder().Build())
	// redirects until the cookie it sets comes back, like many consent pages
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("consent"); err != nil {
			http.SetCookie(w, &http.Cookie{Name: "consent", Value: "1", Path: "/"})
			http.Redirect(w, r, r.URL.Path, http.StatusFound)
			return
		}
		w.Write([]byte(`<p>Subscribe to read on</p>`))
	}))
	defer srv.Close()

	page, err := wrk.fetchPage(srv.URL+"/article", func(string) bool { return false })
	if err != nil || page != "<p>Subscribe to read on </p>" {
		t.Fatalf("expected the page after the redirect, got %q, %v", page, err)
	}
}

func TestFetchPageInternalRedirect(t *testing.T) {
	wrk := NewWorker(nil, client.NewBuilder().Build())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/secret" {
			w.Write([]byte("secret"))
			return
		}
		http.Redirect(w, r, "/secret", http.StatusFound)
	}))
	defer srv.Close()

	// only the redirect target counts as internal here
	internal := func(url string) bool { return strings.HasSuffix(url, "/secret") }
	if page, err := wrk.fetchPage(srv.URL+"/article", internal); err == nil {
		t.Fatalf("expected a redirect to an internal address to be refused, got %q", page)
	}
}
