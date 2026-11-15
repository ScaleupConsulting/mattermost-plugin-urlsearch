package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// ServeHTTP routes the webapp's API; every route requires a logged-in user.
func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()
	router.Use(requireUser)
	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/search", p.handleSearch).Methods(http.MethodGet)
	api.HandleFunc("/links", p.handleLinks).Methods(http.MethodGet)
	router.ServeHTTP(w, r)
}

// requireUser rejects requests the server did not authenticate as a user.
func requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Mattermost-User-Id") == "" {
			writeError(w, http.StatusUnauthorized, "not logged in")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleSearch serves GET /api/v1/search?q=&team_id=&archived=&page=&per_page=.
func (p *Plugin) handleSearch(w http.ResponseWriter, r *http.Request) {
	opts, ok := pageOpts(w, r)
	if !ok {
		return
	}
	page, err := p.Search(r.Header.Get("Mattermost-User-Id"), r.URL.Query().Get("q"), opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// handleLinks serves GET /api/v1/links?team_id=&channel_id=&domain=&archived=&page=&per_page=;
// without channel_id it lists the whole team.
func (p *Plugin) handleLinks(w http.ResponseWriter, r *http.Request) {
	opts, ok := pageOpts(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	channelID := q.Get("channel_id")
	if channelID != "" && !model.IsValidId(channelID) {
		writeError(w, http.StatusBadRequest, "invalid channel_id")
		return
	}
	page, err := p.ListLinks(r.Header.Get("Mattermost-User-Id"), channelID, q.Get("domain"), opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// pageOpts reads the query parameters shared by both routes, answering 400 on bad input.
func pageOpts(w http.ResponseWriter, r *http.Request) (PageOpts, bool) {
	q := r.URL.Query()
	opts := PageOpts{CurrentTeamID: q.Get("team_id"), IncludeArchived: q.Get("archived") == "true"}
	if !model.IsValidId(opts.CurrentTeamID) {
		writeError(w, http.StatusBadRequest, "team_id is required")
		return opts, false
	}
	for name, dst := range map[string]*int{"page": &opts.Page, "per_page": &opts.PerPage} {
		if v := q.Get(name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				writeError(w, http.StatusBadRequest, "invalid "+name)
				return opts, false
			}
			*dst = n
		}
	}
	return opts, true
}

// writeJSON sends v as a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError sends {"error": msg}, the shape the webapp shows to users.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
