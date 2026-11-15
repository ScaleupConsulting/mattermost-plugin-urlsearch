package main

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// migrations are applied in order, once each; append new ones, never edit applied ones.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS urlsearch_links (
		post_id    varchar(26) NOT NULL,
		seq        integer     NOT NULL,
		channel_id varchar(26) NOT NULL,
		team_id    varchar(26),
		user_id    varchar(26) NOT NULL,
		create_at  bigint      NOT NULL,
		url        text        NOT NULL,
		host       text        NOT NULL,
		host_rev   text        NOT NULL,
		path       text        NOT NULL,
		PRIMARY KEY (post_id, seq)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_urlsearch_links_channel_create ON urlsearch_links (channel_id, create_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_urlsearch_links_host_rev ON urlsearch_links (host_rev text_pattern_ops)`,
}

// store owns the plugin's urlsearch_links table in the Mattermost Postgres database.
type store struct {
	db *sql.DB
}

// migrate creates the version table if needed and applies every migration not yet recorded.
// Callers hold the cluster mutex so two nodes never migrate at once.
func (s *store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS urlsearch_schema (version integer PRIMARY KEY)`); err != nil {
		return errors.Wrap(err, "create urlsearch_schema")
	}
	var current int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM urlsearch_schema`).Scan(&current); err != nil {
		return errors.Wrap(err, "read schema version")
	}
	for v := current + 1; v <= len(migrations); v++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[v-1]); err != nil {
			_ = tx.Rollback()
			return errors.Wrapf(err, "migration %d", v)
		}
		if _, err := tx.Exec(`INSERT INTO urlsearch_schema (version) VALUES ($1)`, v); err != nil {
			_ = tx.Rollback()
			return errors.Wrapf(err, "record migration %d", v)
		}
		if err := tx.Commit(); err != nil {
			return errors.Wrapf(err, "commit migration %d", v)
		}
	}
	return nil
}

// postRef identifies a post and the message its links were extracted from.
type postRef struct {
	ID        string
	ChannelID string
	TeamID    string
	UserID    string
	CreateAt  int64
	Message   string
}

// replacePostLinks rewrites a post's rows, but only while the post is live and still has the
// message the links came from, so a stale hook or backfill page cannot overwrite a newer edit.
func (s *store) replacePostLinks(p postRef, links []extractedLink) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var current bool
	err = tx.QueryRow(`SELECT true FROM posts WHERE id = $1 AND deleteat = 0 AND message = $2`, p.ID, p.Message).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "check post is current")
	}
	if _, err := tx.Exec(`DELETE FROM urlsearch_links WHERE post_id = $1`, p.ID); err != nil {
		return errors.Wrap(err, "delete old links")
	}
	var teamID any
	if p.TeamID != "" {
		teamID = p.TeamID
	}
	for i, l := range links {
		if _, err := tx.Exec(`INSERT INTO urlsearch_links (post_id, seq, channel_id, team_id, user_id, create_at, url, host, host_rev, path)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			p.ID, i, p.ChannelID, teamID, p.UserID, p.CreateAt, l.URL, l.Host, l.HostRev, l.Path); err != nil {
			return errors.Wrap(err, "insert link")
		}
	}
	return tx.Commit()
}

// deletePostLinks removes every row for a post.
func (s *store) deletePostLinks(postID string) error {
	_, err := s.db.Exec(`DELETE FROM urlsearch_links WHERE post_id = $1`, postID)
	return errors.Wrap(err, "delete links")
}

// linkRow is one stored link the requesting user may see.
type linkRow struct {
	PostID    string
	ChannelID string
	TeamID    string
	UserID    string
	CreateAt  int64
	URL       string
}

// linkQuery describes one permission-scoped lookup; exactly one of the filters is normally set.
type linkQuery struct {
	UserID          string
	TeamID          string // restrict to this team's channels; with IncludeDirect also DMs and GMs
	IncludeDirect   bool
	ChannelID       string
	Fragment        string // case-insensitive substring of the URL
	HostRev         string // reversed domain prefix, see reverseHost
	IncludeArchived bool
	Limit           int
	Offset          int
}

// queryLinks returns links in channels the user is a member of, newest first. Membership is the
// same rule built-in search uses; deleted posts are excluded by joining the live Posts row.
func (s *store) queryLinks(q linkQuery) ([]linkRow, error) {
	where := []string{"cm.userid = $1", "p.deleteat = 0"}
	args := []any{q.UserID}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if !q.IncludeArchived {
		where = append(where, "c.deleteat = 0")
	}
	if q.ChannelID != "" {
		where = append(where, "l.channel_id = "+arg(q.ChannelID))
	}
	if q.TeamID != "" {
		teamClause := "l.team_id = " + arg(q.TeamID)
		if q.IncludeDirect {
			teamClause = "(" + teamClause + " OR l.team_id IS NULL)"
		}
		where = append(where, teamClause)
	}
	if q.Fragment != "" {
		where = append(where, `l.url ILIKE `+arg("%"+escapeLike(q.Fragment)+"%")+` ESCAPE '\'`)
	}
	if q.HostRev != "" {
		where = append(where, `l.host_rev LIKE `+arg(escapeLike(q.HostRev)+"%")+` ESCAPE '\'`)
	}
	query := `SELECT l.post_id, l.channel_id, COALESCE(l.team_id, ''), l.user_id, l.create_at, l.url
		FROM urlsearch_links l
		JOIN channelmembers cm ON cm.channelid = l.channel_id
		JOIN channels c ON c.id = l.channel_id
		JOIN posts p ON p.id = l.post_id
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY l.create_at DESC, l.post_id, l.seq
		LIMIT ` + arg(q.Limit) + ` OFFSET ` + arg(q.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "query links")
	}
	defer rows.Close()
	var out []linkRow
	for rows.Next() {
		var r linkRow
		if err := rows.Scan(&r.PostID, &r.ChannelID, &r.TeamID, &r.UserID, &r.CreateAt, &r.URL); err != nil {
			return nil, errors.Wrap(err, "scan link")
		}
		out = append(out, r)
	}
	return out, errors.Wrap(rows.Err(), "iterate links")
}

// escapeLike escapes LIKE wildcards so user input is matched literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// backfillCursor is the (create_at, id) position of the last post the backfill indexed.
type backfillCursor struct {
	CreateAt int64  `json:"create_at"`
	PostID   string `json:"post_id"`
	Done     bool   `json:"done"`
}

// nextBackfillPage returns up to limit live user posts after the cursor whose message could
// contain a link. The LIKE prefilter only skips posts that certainly have none.
func (s *store) nextBackfillPage(c backfillCursor, limit int) ([]postRef, error) {
	rows, err := s.db.Query(`SELECT p.id, p.channelid, COALESCE(ch.teamid, ''), p.userid, p.createat, p.message
		FROM posts p JOIN channels ch ON ch.id = p.channelid
		WHERE p.createat >= $1 AND NOT (p.createat = $1 AND p.id <= $2)
		  AND p.deleteat = 0 AND p.type NOT LIKE 'system\_%' AND p.type <> 'burn_on_read'
		  AND (p.message LIKE '%://%' OR p.message ILIKE '%www.%')
		ORDER BY p.createat, p.id
		LIMIT $3`, c.CreateAt, c.PostID, limit)
	if err != nil {
		return nil, errors.Wrap(err, "query backfill page")
	}
	defer rows.Close()
	var out []postRef
	for rows.Next() {
		var p postRef
		if err := rows.Scan(&p.ID, &p.ChannelID, &p.TeamID, &p.UserID, &p.CreateAt, &p.Message); err != nil {
			return nil, errors.Wrap(err, "scan backfill post")
		}
		out = append(out, p)
	}
	return out, errors.Wrap(rows.Err(), "iterate backfill page")
}
