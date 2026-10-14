package worker

import (
	"errors"
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

func TestRefetchPages(t *testing.T) {
	db, err := storage.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	w := NewWorker(db, client.NewBuilder().Build())
	feed := db.CreateFeed(model.CreateFeedParams{Title: "feed", FeedLink: "http://example.com/feed.xml"})
	old := "old page"
	db.CreateItems([]model.Item{
		{GUID: "1", FeedId: feed.Id, Title: "one", Link: "http://127.0.0.1/1", Page: &old, Date: time.Now()},
		{GUID: "2", FeedId: feed.Id, Title: "two", Link: "http://127.0.0.1/2", Page: &old, Date: time.Now()},
	})

	if w.FetchMissingPages(db, feed.Id) {
		t.Fatal("expected no pages to be missing")
	}
	if !w.RefetchPages(db, feed.Id) {
		t.Fatal("expected the pages to be fetched again")
	}
	for deadline := time.Now().Add(5 * time.Second); ; {
		if _, ok := w.PagesLeft(feed.Id); !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fetching the pages did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// the links cannot be fetched, which replaces the pages with empty ones
	db.ForEachItem(false, true, func(item model.Item) {
		if item.Page == nil || *item.Page != "" {
			t.Errorf("expected the page of %s to be fetched again, got %v", item.Title, item.Page)
		}
	})
}

func TestFetchPageChallenge(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		header  string
		body    string
		blocked bool
	}{
		{"cloudflare header", 403, "challenge", "<title>Just a moment...</title>", true},
		{"cloudflare script", 403, "", `<script src="/cdn-cgi/challenge-platform/h/b/orchestrate/chl_page/v1"></script>`, true},
		{"datadome", 403, "", `<script src="https://ct.captcha-delivery.com/c.js"></script>`, true},
		{"captcha on error page", 429, "", `<script src="https://www.google.com/recaptcha/api.js"></script>`, true},
		{"captcha on article", 200, "", `<p>news</p><script src="https://www.google.com/recaptcha/api.js"></script>`, false},
		{"paywall", 402, "", `<p>Subscribe to read on</p>`, false},
	}
	wrk := NewWorker(nil, client.NewBuilder().Build())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.header != "" {
					w.Header().Set("Cf-Mitigated", tt.header)
				}
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			page, err := wrk.fetchPage(srv.URL, func(string) bool { return false })
			if blocked := errors.Is(err, ErrBlocked); blocked != tt.blocked {
				t.Fatalf("expected blocked %v, got %q, %v", tt.blocked, page, err)
			}
		})
	}
}
