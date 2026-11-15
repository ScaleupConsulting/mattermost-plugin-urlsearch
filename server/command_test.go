package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCommand(t *testing.T) {
	a, err := parseCommand(strings.Fields("example.com --team --archived --page 3"))
	require.NoError(t, err)
	assert.Equal(t, commandArgs{Text: "example.com", Team: true, Archived: true, Page: 2}, a)

	a, err = parseCommand(strings.Fields("docs.google.com/spreadsheets --page=2"))
	require.NoError(t, err)
	assert.Equal(t, commandArgs{Text: "docs.google.com/spreadsheets", Page: 1}, a)

	for _, bad := range []string{"--page", "--page 0", "--page x", "--bogus"} {
		_, err := parseCommand(strings.Fields(bad))
		assert.Error(t, err, bad)
	}
}

func TestNextCommand(t *testing.T) {
	assert.Equal(t, "/links github.com --team --page 3", nextCommand(linksTrigger, commandArgs{Text: "github.com", Team: true, Page: 1}))
}

func TestFormatPage(t *testing.T) {
	out := formatPage("Links", &LinkPage{Page: 0, PerPage: 20, HasMore: true, Links: []Link{{URL: "https://x.test", ChannelName: "~town", Username: "alice", Permalink: "http://mm/t/pl/p1"}}}, "/urlsearch x --page 2")
	assert.Contains(t, out, "1. https://x.test")
	assert.Contains(t, out, "[jump to post](http://mm/t/pl/p1)")
	assert.Contains(t, out, "More results: `/urlsearch x --page 2`")
}
