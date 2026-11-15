package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// isIndexable reports whether a post's links belong in the index: live, user-authored, not burn-on-read.
func isIndexable(post *model.Post) bool {
	return post.DeleteAt == 0 && !post.IsSystemMessage() && post.Type != model.PostTypeBurnOnRead
}

// MessageHasBeenPosted indexes the links in a new post.
func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	p.indexPost(post)
}

// MessageHasBeenUpdated replaces an edited post's links, removing them if it no longer has any.
func (p *Plugin) MessageHasBeenUpdated(_ *plugin.Context, newPost, _ *model.Post) {
	p.indexPost(newPost)
}

// MessageHasBeenDeleted drops a deleted post's links.
func (p *Plugin) MessageHasBeenDeleted(_ *plugin.Context, post *model.Post) {
	if err := p.store.deletePostLinks(post.Id); err != nil {
		p.API.LogError("Failed to remove links of deleted post", "post_id", post.Id, "err", err.Error())
	}
}

// indexPost writes a post's current links; a post that is not indexable just loses its rows.
func (p *Plugin) indexPost(post *model.Post) {
	if !isIndexable(post) {
		if err := p.store.deletePostLinks(post.Id); err != nil {
			p.API.LogError("Failed to remove links of unindexable post", "post_id", post.Id, "err", err.Error())
		}
		return
	}
	ch, appErr := p.API.GetChannel(post.ChannelId)
	if appErr != nil {
		p.API.LogError("Failed to read channel for link indexing", "post_id", post.Id, "err", appErr.Error())
		return
	}
	ref := postRef{ID: post.Id, ChannelID: post.ChannelId, TeamID: ch.TeamId, UserID: post.UserId, CreateAt: post.CreateAt, Message: post.Message}
	if err := p.store.replacePostLinks(ref, extractLinks(post.Message)); err != nil {
		p.API.LogError("Failed to index post links", "post_id", post.Id, "err", err.Error())
	}
}
