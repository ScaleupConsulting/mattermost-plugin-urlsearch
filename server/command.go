package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	searchTrigger = "urlsearch"
	linksTrigger  = "links"
)

// commands returns the two slash commands, which are the search surface on every client, mobile included.
func commands() []*model.Command {
	return []*model.Command{
		{
			Trigger:          searchTrigger,
			DisplayName:      "URL search",
			Description:      "Find links containing a piece of a URL in your channels, DMs and group messages.",
			AutoComplete:     true,
			AutoCompleteHint: "<part of a URL> [--archived] [--page N]",
			AutoCompleteDesc: "Find links containing this text. System admins: \"backfill\" reindexes every post.",
		},
		{
			Trigger:          linksTrigger,
			DisplayName:      "Channel links",
			Description:      "List the links posted in this channel, or the whole team with --team, optionally for one domain.",
			AutoComplete:     true,
			AutoCompleteHint: "[domain] [--team] [--archived] [--page N]",
			AutoCompleteDesc: "List links in this channel; a domain also matches its subdomains.",
		},
	}
}

// commandArgs is a parsed /urlsearch or /links line.
type commandArgs struct {
	Text     string // the fragment or domain: every word that is not a flag
	Team     bool
	Archived bool
	Page     int // zero-based
}

// parseCommand splits the words after the trigger into flags and text; --page is one-based for users.
func parseCommand(words []string) (commandArgs, error) {
	var a commandArgs
	var text []string
	for i := 0; i < len(words); i++ {
		w := words[i]
		switch {
		case w == "--team":
			a.Team = true
		case w == "--archived":
			a.Archived = true
		case w == "--page" || strings.HasPrefix(w, "--page="):
			v := strings.TrimPrefix(w, "--page=")
			if w == "--page" {
				if i+1 >= len(words) {
					return a, fmt.Errorf("--page needs a number")
				}
				i++
				v = words[i]
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return a, fmt.Errorf("--page needs a number from 1")
			}
			a.Page = n - 1
		case strings.HasPrefix(w, "--"):
			return a, fmt.Errorf("unknown option %s", w)
		default:
			text = append(text, w)
		}
	}
	a.Text = strings.Join(text, " ")
	return a, nil
}

// ExecuteCommand answers /urlsearch and /links with an ephemeral list of permalinks.
func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	fields := strings.Fields(args.Command)
	if len(fields) == 0 {
		return ephemeral("Empty command."), nil
	}
	trigger := strings.TrimPrefix(fields[0], "/")
	words := fields[1:]

	if trigger == searchTrigger && len(words) == 1 && words[0] == "backfill" {
		return ephemeral(p.backfillCommand(args.UserId)), nil
	}
	a, err := parseCommand(words)
	if err != nil {
		return ephemeral(err.Error()), nil
	}
	opts := PageOpts{CurrentTeamID: args.TeamId, IncludeArchived: a.Archived, Page: a.Page, PerPage: defaultPerPage}

	switch trigger {
	case searchTrigger:
		if a.Team {
			return ephemeral("--team applies to /links only; /urlsearch already covers the team plus your DMs and group messages."), nil
		}
		if a.Text == "" {
			return ephemeral("Usage: /urlsearch <part of a URL> [--archived] [--page N]"), nil
		}
		page, err := p.Search(args.UserId, a.Text, opts)
		if err != nil {
			return ephemeral(err.Error()), nil
		}
		return ephemeral(formatPage(fmt.Sprintf("Links containing `%s`", sanitizeCode(a.Text)), page, nextCommand(searchTrigger, a))), nil
	case linksTrigger:
		channelID := args.ChannelId
		scope := "this channel"
		if a.Team {
			channelID = ""
			scope = "this team"
		}
		page, err := p.ListLinks(args.UserId, channelID, a.Text, opts)
		if err != nil {
			return ephemeral(err.Error()), nil
		}
		title := "Links in " + scope
		if d := normalizeDomain(a.Text); d != "" {
			title += fmt.Sprintf(" on `%s`", sanitizeCode(d))
		}
		return ephemeral(formatPage(title, page, nextCommand(linksTrigger, a))), nil
	}
	return ephemeral("Unknown command."), nil
}

// backfillCommand restarts the backfill from the first post; system admins only.
func (p *Plugin) backfillCommand(userID string) string {
	if !p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		return "Only system admins can run the backfill."
	}
	started, err := p.startBackfill(true)
	if err != nil {
		return "Could not start the backfill: " + err.Error()
	}
	if !started {
		return "A backfill is already running on this server."
	}
	return "Backfill started. It reindexes every post and logs \"Link backfill complete\" when done."
}

// formatPage renders one page as a numbered markdown list with a hint for the next page.
func formatPage(title string, page *LinkPage, next string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s** (page %d)\n", title, page.Page+1)
	if len(page.Links) == 0 {
		b.WriteString("No links found.")
		return b.String()
	}
	for i, l := range page.Links {
		fmt.Fprintf(&b, "%d. %s\n    %s by @%s on %s UTC, [jump to post](%s)\n",
			page.Page*page.PerPage+i+1, l.URL, l.ChannelName, l.Username,
			time.UnixMilli(l.CreateAt).UTC().Format("2006-01-02 15:04"), l.Permalink)
	}
	if page.HasMore {
		fmt.Fprintf(&b, "\nMore results: `%s`", next)
	}
	return b.String()
}

// nextCommand is the command line that shows the following page.
func nextCommand(trigger string, a commandArgs) string {
	parts := []string{"/" + trigger}
	if a.Text != "" {
		parts = append(parts, sanitizeCode(a.Text))
	}
	if a.Team {
		parts = append(parts, "--team")
	}
	if a.Archived {
		parts = append(parts, "--archived")
	}
	parts = append(parts, "--page", strconv.Itoa(a.Page+2))
	return strings.Join(parts, " ")
}

// sanitizeCode keeps user text from closing the inline code span it is shown in.
func sanitizeCode(s string) string {
	return strings.ReplaceAll(s, "`", "'")
}

// ephemeral wraps text as a reply only the caller sees.
func ephemeral(text string) *model.CommandResponse {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: text}
}
