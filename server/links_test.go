package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
)

type fakeChecker struct {
	allowed map[string]bool
	calls   map[string]int
}

func (f *fakeChecker) HasPermissionToChannel(userID, channelID string, permission *model.Permission) bool {
	f.calls[channelID]++
	return userID == "u1" && permission == model.PermissionReadChannelContent && f.allowed[channelID]
}

func TestFilterPermittedDropsUnreadableChannelsAndChecksEachOnce(t *testing.T) {
	f := &fakeChecker{allowed: map[string]bool{"pub": true, "dm": true}, calls: map[string]int{}}
	rows := []linkRow{
		{PostID: "1", ChannelID: "pub"},
		{PostID: "2", ChannelID: "priv"},
		{PostID: "3", ChannelID: "pub"},
		{PostID: "4", ChannelID: "dm"},
		{PostID: "5", ChannelID: "priv"},
	}
	got := filterPermitted(f, "u1", rows)
	var ids []string
	for _, r := range got {
		ids = append(ids, r.PostID)
	}
	assert.Equal(t, []string{"1", "3", "4"}, ids)
	assert.Equal(t, map[string]int{"pub": 1, "priv": 1, "dm": 1}, f.calls)
}

func TestFilterPermittedOtherUserSeesNothing(t *testing.T) {
	f := &fakeChecker{allowed: map[string]bool{"pub": true}, calls: map[string]int{}}
	assert.Empty(t, filterPermitted(f, "u2", []linkRow{{PostID: "1", ChannelID: "pub"}}))
}

func TestIsIndexable(t *testing.T) {
	assert.True(t, isIndexable(&model.Post{}))
	assert.False(t, isIndexable(&model.Post{DeleteAt: 1}))
	assert.False(t, isIndexable(&model.Post{Type: model.PostTypeJoinChannel}))
	assert.False(t, isIndexable(&model.Post{Type: model.PostTypeBurnOnRead}))
}
