package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/nkanaev/yarr/src/content/htmlutil"
	"github.com/nkanaev/yarr/src/content/readability"
	"github.com/nkanaev/yarr/src/content/sanitizer"
	"github.com/nkanaev/yarr/src/content/silo"
	"github.com/nkanaev/yarr/src/filter"
	"github.com/nkanaev/yarr/src/parser/opml"
	"github.com/nkanaev/yarr/src/server/middleware"
	"github.com/nkanaev/yarr/src/storage/model"
	"github.com/nkanaev/yarr/src/worker"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("failed to write JSON:", err)
	}
}

func writeHTML(w http.ResponseWriter, status int, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	if err := tmpl.Execute(w, data); err != nil {
		log.Println("failed to write HTML:", err)
	}
}

func (s *Server) Handler() http.Handler {
	staticFS := http.FileServer(http.FS(s.StaticFS))

	secure := func(next http.HandlerFunc) http.HandlerFunc {
		if s.Auth == nil {
			return next
		}
		return s.Auth.Middleware(next)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", s.handleIndex)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/static/{path...}", http.StripPrefix("/static/", staticFS).ServeHTTP)
	mux.HandleFunc("/fever/", s.handleFever)
	mux.HandleFunc("/manifest.json", s.handleManifest)
	mux.HandleFunc("/api/status", secure(s.handleStatus))
	mux.HandleFunc("/api/folders", secure(s.handleFolderList))
	mux.HandleFunc("/api/folders/{id}", secure(s.handleFolder))
	mux.HandleFunc("/api/feeds", secure(s.handleFeedList))
	mux.HandleFunc("/api/feeds/refresh", secure(s.handleFeedRefresh))
	mux.HandleFunc("/api/feeds/errors", secure(s.handleFeedErrors))
	mux.HandleFunc("/api/feeds/{id}", secure(s.handleFeed))
	mux.HandleFunc("/api/items", secure(s.handleItemList))
	mux.HandleFunc("/api/items/{id}", secure(s.handleItem))
	mux.HandleFunc("/api/filters", secure(s.handleFilterList))
	mux.HandleFunc("/api/filters/preview", secure(s.handleFilterPreview))
	mux.HandleFunc("/api/filters/{id}", secure(s.handleFilter))
	mux.HandleFunc("/api/filters/{id}/refetch", secure(s.handleFilterRefetch))
	mux.HandleFunc("/api/settings", secure(s.handleSettings))
	mux.HandleFunc("/opml/import", secure(s.handleOPMLImport))
	mux.HandleFunc("/opml/export", secure(s.handleOPMLExport))
	mux.HandleFunc("/page", secure(s.handlePageCrawl))
	mux.HandleFunc("/logout", secure(s.handleLogout))

	handler := middleware.Base(mux, s.BasePath)

	return middleware.Gzip(handler)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	isAuthenticated := false
	requiresAuth := false
	if s.Auth == nil {
		isAuthenticated = true
	} else {
		requiresAuth = true
		isAuthenticated = s.Auth.IsAuthenticated(r)
	}

	settings := model.SettingsDefault()
	if isAuthenticated {
		settings = s.db(r).GetSettings()
	}

	writeHTML(w, http.StatusOK, s.Template.Lookup("index.html"), map[string]any{
		"settings":      settings.Map(),
		"authenticated": isAuthenticated,
		"requiresAuth":  requiresAuth,
	})
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"$schema":     "https://json.schemastore.org/web-manifest-combined.json",
		"name":        "yarr!",
		"short_name":  "yarr",
		"description": "yet another rss reader",
		"display":     "standalone",
		"start_url":   "/" + strings.TrimPrefix(s.BasePath, "/"),
		"icons": []map[string]any{
			{
				"src":   s.BasePath + "/static/favicon.png",
				"sizes": "64x64",
				"type":  "image/png",
			},
		},
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var running int32
	if s.Scheduler != nil {
		running = s.Scheduler.FeedsPending()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"running": running,
		"refresh": s.Scheduler != nil,
		"stats":   s.db(r).FeedStats(),
	})
}

func (s *Server) handleFolderList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.db(r).ListFolders())
	case http.MethodPost:
		var body FolderCreateForm
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(body.Title) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Folder title missing."})
			return
		}
		writeJSON(w, http.StatusCreated, s.db(r).CreateFolder(body.Title))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body FolderUpdateForm
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.db(r).UpdateFolder(id, model.UpdateFolderParams{
			Title:      body.Title,
			IsExpanded: body.IsExpanded,
		})
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		s.db(r).DeleteFolder(id)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFeedRefresh(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if s.Scheduler != nil {
			s.Scheduler.RefreshFeeds()
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFeedErrors(w http.ResponseWriter, r *http.Request) {
	errors := make(map[int64]string)
	states, err := s.db(r).ListFeedStates()
	if err == nil {
		for _, state := range states {
			if state.LastError != "" {
				errors[state.FeedID] = state.LastError
			}
		}
	}
	writeJSON(w, http.StatusOK, errors)
}

func (s *Server) handleFeedList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.db(r).ListFeeds())
	case http.MethodPost:
		var form FeedCreateForm
		if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		result, err := worker.DiscoverFeed(form.Url)
		switch {
		case err != nil:
			log.Printf("Faild to discover feed for %s: %s", form.Url, err)
			writeJSON(w, http.StatusOK, map[string]string{"status": "notfound"})
		case len(result.Sources) > 0:
			writeJSON(
				w,
				http.StatusOK,
				map[string]any{"status": "multiple", "choice": result.Sources},
			)
		case result.Feed != nil:
			title := result.Feed.Title
			if form.TitleOverride != "" {
				title = form.TitleOverride
			}
			feed := s.db(r).CreateFeed(model.CreateFeedParams{
				Title:    title,
				Link:     result.Feed.SiteURL,
				FeedLink: result.FeedLink,
				FolderID: form.FolderID,
			})
			items := worker.ConvertItems(result.Feed.Items, *feed)
			if len(items) > 0 {
				filter.ApplyStored(s.db(r), items)
				s.db(r).CreateItems(items)
			}
			// TODO: DiscoverFeed must search for favicon too

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "success",
				"feed":   feed,
			})
		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "notfound"})
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		feed := s.db(r).GetFeed(id)
		if feed == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body := make(map[string]any)
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		params := model.UpdateFeedParams{}
		if title, ok := body["title"]; ok {
			if reflect.TypeOf(title).Kind() == reflect.String {
				t := title.(string)
				params.Title = &t
			}
		}
		if f_id, ok := body["folder_id"]; ok {
			if f_id == nil {
				params.FolderID = model.SetNullable[int64](nil)
			} else if reflect.TypeOf(f_id).Kind() == reflect.Float64 {
				folderId := int64(f_id.(float64))
				params.FolderID = model.SetNullable(&folderId)
			}
		}
		if link, ok := body["feed_link"]; ok {
			if reflect.TypeOf(link).Kind() == reflect.String {
				l := link.(string)
				params.FeedLink = &l
			}
		}
		s.db(r).UpdateFeed(id, params)
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		s.db(r).DeleteFeed(id)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		item := s.db(r).GetItem(id)
		if item == nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// runtime fix for relative links
		if !htmlutil.IsAPossibleLink(item.Link) {
			if feed := s.db(r).GetFeed(item.FeedId); feed != nil {
				item.Link = htmlutil.AbsoluteUrl(item.Link, feed.Link)
			}
		}

		item.Content = sanitizer.Sanitize(item.Link, item.Content)
		for i, link := range item.MediaLinks {
			item.MediaLinks[i].Description = sanitizer.Sanitize(item.Link, link.Description)
		}

		writeJSON(w, http.StatusOK, item)
	case http.MethodPut:
		var body ItemUpdateForm
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Status != nil {
			s.db(r).UpdateItem(id, model.UpdateItemParams{Status: body.Status})
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleItemList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		perPage := 20
		query := r.URL.Query()

		filter := model.ItemFilter{}
		if folderID, err := strconv.ParseInt(query.Get("folder_id"), 10, 64); err == nil {
			filter.FolderID = &folderID
		}
		if feedID, err := strconv.ParseInt(query.Get("feed_id"), 10, 64); err == nil {
			filter.FeedID = &feedID
		}
		if after, err := strconv.ParseInt(query.Get("after"), 10, 64); err == nil {
			filter.After = &after
		}
		if status := query.Get("status"); len(status) != 0 {
			statusValue := model.StatusValues[status]
			filter.Status = &statusValue
		}
		if search := query.Get("search"); len(search) != 0 {
			filter.Search = &search
		}
		newestFirst := query.Get("oldest_first") != "true"

		items := s.db(r).ListItems(filter, perPage+1, newestFirst, true)
		hasMore := false
		if len(items) == perPage+1 {
			hasMore = true
			items = items[:perPage]
		}

		for i, item := range items {
			if item.Title == "" {
				text := htmlutil.ExtractText(item.Content)
				items[i].Title = htmlutil.TruncateText(text, 140)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"list":     items,
			"has_more": hasMore,
		})
	case http.MethodPut:
		filter := model.MarkFilter{}

		query := r.URL.Query()
		if folderID, err := strconv.ParseInt(query.Get("folder_id"), 10, 64); err == nil {
			filter.FolderID = &folderID
		}
		if feedID, err := strconv.ParseInt(query.Get("feed_id"), 10, 64); err == nil {
			filter.FeedID = &feedID
		}
		s.db(r).MarkItemsRead(filter)
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.db(r).GetSettings())
	case http.MethodPut:
		var params model.UpdateSettingsParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if s.db(r).UpdateSettings(params) {
			if s.Scheduler != nil && params.RefreshRate != nil {
				s.Scheduler.SetRefreshRate(*params.RefreshRate)
			}
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOPMLImport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		file, _, err := r.FormFile("opml")
		if err != nil {
			log.Print(err)
			return
		}
		doc, err := opml.Parse(file)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, f := range doc.Feeds {
			s.db(r).CreateFeed(model.CreateFeedParams{
				Title:    f.Title,
				Link:     f.SiteUrl,
				FeedLink: f.FeedUrl,
			})
		}
		for _, f := range doc.Folders {
			folder := s.db(r).CreateFolder(f.Title)
			for _, ff := range f.AllFeeds() {
				s.db(r).CreateFeed(model.CreateFeedParams{
					Title:    ff.Title,
					Link:     ff.SiteUrl,
					FeedLink: ff.FeedUrl,
					FolderID: &folder.Id,
				})
			}
		}

		if s.Scheduler != nil {
			s.Scheduler.RefreshFeeds()
		}

		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOPMLExport(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		filename := fmt.Sprintf("subscriptions_%s.opml", time.Now().Format("2006-01-02_15-04-05"))
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		doc := opml.Folder{}

		feedsByFolderID := make(map[int64][]*model.Feed)
		for _, feed := range s.db(r).ListFeeds() {
			if feed.FolderId == nil {
				doc.Feeds = append(doc.Feeds, opml.Feed{
					Title:   feed.Title,
					FeedUrl: feed.FeedLink,
					SiteUrl: feed.Link,
				})
			} else {
				id := *feed.FolderId
				feedsByFolderID[id] = append(feedsByFolderID[id], &feed)
			}
		}

		for _, folder := range s.db(r).ListFolders() {
			folderFeeds := feedsByFolderID[folder.Id]
			if len(folderFeeds) == 0 {
				continue
			}
			opmlfolder := opml.Folder{Title: folder.Title}
			for _, feed := range folderFeeds {
				opmlfolder.Feeds = append(opmlfolder.Feeds, opml.Feed{
					Title:   feed.Title,
					FeedUrl: feed.FeedLink,
					SiteUrl: feed.Link,
				})
			}
			doc.Folders = append(doc.Folders, opmlfolder)
		}

		w.Write([]byte(doc.OPML()))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePageCrawl(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	url = silo.RedirectURL(url)

	if content := silo.VideoIFrame(url); content != "" {
		writeJSON(w, http.StatusOK, map[string]string{
			"content": sanitizer.Sanitize(url, content),
		})
		return
	}
	if worker.IsInternalURL(url) {
		log.Printf("attempt to access internal IP %s from %s", url, r.RemoteAddr)
		return
	}

	body, err := worker.GetBody(url)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	content, err := readability.ExtractContent(strings.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"content": "error: " + err.Error(),
		})
		return
	}
	content = sanitizer.Sanitize(url, content)
	writeJSON(w, http.StatusOK, map[string]string{
		"content": content,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		username := r.FormValue("username")
		password := r.FormValue("password")
		if s.Auth == nil || !s.Auth.Authenticate(w, username, password) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.Auth != nil {
		s.Auth.Logout(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

const filterPreviewLimit = 50

func decodeFilterParams(w http.ResponseWriter, r *http.Request) (model.FilterParams, bool) {
	var params model.FilterParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return params, false
	}
	if err := filter.Check(params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return params, false
	}
	return params, true
}

// refilter applies changed filters to the stored items and returns the
// filter with the given id, as stored. Pages missing for filters checking
// them are fetched in the background.
func (s *Server) refilter(r *http.Request, id int64) *model.Filter {
	if err := filter.Refilter(s.db(r)); err != nil {
		log.Print(err)
	}
	for _, feedId := range filter.New(s.db(r).ListFilters()).PageFeeds() {
		worker.FetchMissingPages(s.db(r), feedId)
	}
	for _, f := range s.db(r).ListFilters() {
		if f.Id == id {
			return &f
		}
	}
	return nil
}

func (s *Server) handleFilterList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.db(r).ListFilters())
	case http.MethodPost:
		params, ok := decodeFilterParams(w, r)
		if !ok {
			return
		}
		created := s.db(r).CreateFilter(params)
		if created == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, s.refilter(r, created.Id))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFilter(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		params, ok := decodeFilterParams(w, r)
		if !ok {
			return
		}
		if !s.db(r).UpdateFilter(id, params) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		updated := s.refilter(r, id)
		if updated == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		if !s.db(r).DeleteFilter(id) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.refilter(r, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFilterPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var params model.FilterParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// an invalid pattern is expected while typing, so it is not a failed request
	count, matches, err := filter.Preview(s.db(r), params, filterPreviewLimit)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error(), "count": 0, "list": []any{}})
		return
	}
	result := map[string]any{"count": count, "list": matches}
	if params.MatchPage {
		// matches grow while the pages are fetched, so the preview is asked again until done
		worker.FetchMissingPages(s.db(r), *params.FeedId)
		left, fetching := worker.PagesLeft(*params.FeedId)
		result["pages"] = map[string]any{"fetching": fetching, "left": left}
	}
	writeJSON(w, http.StatusOK, result)
}

// handleFilterRefetch fetches the article pages checked by a filter again,
// e.g. after fetching them failed, and re-applies the filters as they arrive.
// The preview reports the progress.
func (s *Server) handleFilterRefetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var found *model.Filter
	for _, f := range s.db(r).ListFilters() {
		if f.Id == id {
			found = &f
		}
	}
	switch {
	case found == nil:
		w.WriteHeader(http.StatusNotFound)
	case !found.MatchPage || found.FeedId == nil:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "filter does not check the article page"})
	case !worker.RefetchPages(s.db(r), *found.FeedId):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "pages are being fetched already"})
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}
