package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

const (
	defaultPerPage = 20
	maxPerPage     = 100
)

// Link is one search or list result as both the slash commands and the webapp see it.
type Link struct {
	PostID      string `json:"post_id"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	CreateAt    int64  `json:"create_at"`
	URL         string `json:"url"`
	Permalink   string `json:"permalink"`
}

// LinkPage is one page of results; HasMore says whether the next page has anything.
type LinkPage struct {
	Links   []Link `json:"links"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	HasMore bool   `json:"has_more"`
}

// PageOpts are the options common to Search and ListLinks.
type PageOpts struct {
	// CurrentTeamID scopes the query and names the team used in DM and GM permalinks.
	CurrentTeamID   string
	IncludeArchived bool
	Page            int
	PerPage         int
}

// permissionChecker is the part of the plugin API the result filter needs; tests fake it.
type permissionChecker interface {
	HasPermissionToChannel(userID, channelID string, permission *model.Permission) bool
}

// Search finds links containing fragment in channels of the current team plus DMs and GMs.
func (p *Plugin) Search(userID, fragment string, opts PageOpts) (*LinkPage, error) {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return nil, errors.New("enter part of a URL to search for")
	}
	return p.runQuery(linkQuery{UserID: userID, TeamID: opts.CurrentTeamID, IncludeDirect: true, Fragment: fragment}, opts)
}

// ListLinks lists links in one channel, or in every channel of the current team when channelID
// is empty, optionally limited to a domain and its subdomains.
func (p *Plugin) ListLinks(userID, channelID, domain string, opts PageOpts) (*LinkPage, error) {
	q := linkQuery{UserID: userID, ChannelID: channelID}
	if channelID == "" {
		q.TeamID = opts.CurrentTeamID
	}
	if d := normalizeDomain(domain); d != "" {
		q.HostRev = reverseHost(d)
	}
	return p.runQuery(q, opts)
}

// runQuery applies paging, runs the membership-scoped query, rechecks channel permissions and
// fills in names and permalinks.
func (p *Plugin) runQuery(q linkQuery, opts PageOpts) (*LinkPage, error) {
	if opts.CurrentTeamID == "" {
		return nil, errors.New("the current team is required")
	}
	if opts.PerPage <= 0 || opts.PerPage > maxPerPage {
		opts.PerPage = defaultPerPage
	}
	if opts.Page < 0 {
		opts.Page = 0
	}
	q.IncludeArchived = opts.IncludeArchived
	q.Limit = opts.PerPage + 1
	q.Offset = opts.Page * opts.PerPage
	rows, err := p.store.queryLinks(q)
	if err != nil {
		return nil, err
	}
	page := &LinkPage{Page: opts.Page, PerPage: opts.PerPage, Links: []Link{}}
	if len(rows) > opts.PerPage {
		page.HasMore = true
		rows = rows[:opts.PerPage]
	}
	rows = filterPermitted(p.API, q.UserID, rows)
	n := newNamer(p, opts.CurrentTeamID, q.UserID)
	for _, r := range rows {
		page.Links = append(page.Links, Link{
			PostID:      r.PostID,
			ChannelID:   r.ChannelID,
			ChannelName: n.channel(r.ChannelID),
			UserID:      r.UserID,
			Username:    n.user(r.UserID),
			CreateAt:    r.CreateAt,
			URL:         r.URL,
			Permalink:   n.permalink(r),
		})
	}
	return page, nil
}

// filterPermitted drops rows in channels where the user lacks read permission, checking each
// channel once. The SQL already requires membership; this applies the server's own rule too.
func filterPermitted(api permissionChecker, userID string, rows []linkRow) []linkRow {
	allowed := map[string]bool{}
	out := rows[:0]
	for _, r := range rows {
		ok, seen := allowed[r.ChannelID]
		if !seen {
			ok = api.HasPermissionToChannel(userID, r.ChannelID, model.PermissionReadChannelContent)
			allowed[r.ChannelID] = ok
		}
		if ok {
			out = append(out, r)
		}
	}
	return out
}

// namer resolves channel, user and team names once per request.
type namer struct {
	p             *Plugin
	siteURL       string
	currentTeamID string
	viewerID      string
	channels      map[string]*model.Channel
	users         map[string]string
	teams         map[string]string
}

// newNamer starts an empty per-request name cache.
func newNamer(p *Plugin, currentTeamID, viewerID string) *namer {
	siteURL := ""
	if cfg := p.API.GetConfig(); cfg != nil && cfg.ServiceSettings.SiteURL != nil {
		siteURL = strings.TrimRight(*cfg.ServiceSettings.SiteURL, "/")
	}
	return &namer{p: p, siteURL: siteURL, currentTeamID: currentTeamID, viewerID: viewerID,
		channels: map[string]*model.Channel{}, users: map[string]string{}, teams: map[string]string{}}
}

// getChannel returns the channel or nil, cached.
func (n *namer) getChannel(id string) *model.Channel {
	if ch, ok := n.channels[id]; ok {
		return ch
	}
	ch, appErr := n.p.API.GetChannel(id)
	if appErr != nil {
		ch = nil
	}
	n.channels[id] = ch
	return ch
}

// channel names a channel the way a reader recognises it: ~name, @other for a DM, or the GM's members.
func (n *namer) channel(id string) string {
	ch := n.getChannel(id)
	switch {
	case ch == nil:
		return id
	case ch.Type == model.ChannelTypeDirect:
		other := ch.GetOtherUserIdForDM(n.viewerID)
		if other == "" {
			other = n.viewerID
		}
		return "@" + n.user(other) + " (direct message)"
	case ch.Type == model.ChannelTypeGroup:
		return ch.DisplayName + " (group message)"
	default:
		return "~" + ch.Name
	}
}

// user returns a username, cached.
func (n *namer) user(id string) string {
	if name, ok := n.users[id]; ok {
		return name
	}
	name := id
	if u, appErr := n.p.API.GetUser(id); appErr == nil {
		name = u.Username
	}
	n.users[id] = name
	return name
}

// team returns a team's URL name, cached.
func (n *namer) team(id string) string {
	if name, ok := n.teams[id]; ok {
		return name
	}
	name := ""
	if t, appErr := n.p.API.GetTeam(id); appErr == nil {
		name = t.Name
	}
	n.teams[id] = name
	return name
}

// permalink links to the post under its own team, or under the current team for DMs and GMs.
func (n *namer) permalink(r linkRow) string {
	teamID := r.TeamID
	if teamID == "" {
		teamID = n.currentTeamID
	}
	return n.siteURL + "/" + n.team(teamID) + "/pl/" + r.PostID
}
