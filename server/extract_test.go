package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractLinks(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    []string
	}{
		{"bare autolink", "see https://foo.example.com/a/b?x=1 now", []string{"https://foo.example.com/a/b?x=1"}},
		{"trailing period", "go to https://example.com/x.", []string{"https://example.com/x"}},
		{"trailing comma", "https://example.com/a, and more", []string{"https://example.com/a"}},
		{"wrapped in parens", "(see https://example.com/doc)", []string{"https://example.com/doc"}},
		{"balanced parens kept", "https://en.wikipedia.org/wiki/Foo_(bar)", []string{"https://en.wikipedia.org/wiki/Foo_(bar)"}},
		{"markdown link", "[doc](https://example.org/x) here", []string{"https://example.org/x"}},
		{"reference link", "[doc][1]\n\n[1]: https://ref.example.net/p", []string{"https://ref.example.net/p"}},
		{"www autolink", "visit www.example.com/path", []string{"http://www.example.com/path"}},
		{"dedupe", "https://a.example.com https://a.example.com", []string{"https://a.example.com"}},
		{"several in order", "https://one.test/1 then [two](http://two.test/2)", []string{"https://one.test/1", "http://two.test/2"}},
		{"code span skipped", "`https://code.example.com` and https://real.example.com", []string{"https://real.example.com"}},
		{"code block skipped", "```\nhttps://block.example.com\n```", nil},
		{"mailto skipped", "<mailto:a@example.com>", nil},
		{"no links", "plain text", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, l := range extractLinks(c.message) {
				got = append(got, l.URL)
			}
			assert.Equal(t, c.want, got)
		})
	}
}

func TestParseLinkParts(t *testing.T) {
	l, ok := parseLink("https://Docs.Google.com/spreadsheets/d/abc?usp=sharing")
	assert.True(t, ok)
	assert.Equal(t, "docs.google.com", l.Host)
	assert.Equal(t, "com.google.docs.", l.HostRev)
	assert.Equal(t, "/spreadsheets/d/abc?usp=sharing", l.Path)

	_, ok = parseLink("ftp://example.com/x")
	assert.False(t, ok)
}

func TestReverseHostMatchesSubdomainsOnly(t *testing.T) {
	prefix := reverseHost("example.com")
	assert.Equal(t, "com.example.", prefix)
	assert.True(t, hasPrefix(reverseHost("www.example.com"), prefix))
	assert.True(t, hasPrefix(reverseHost("example.com"), prefix))
	assert.False(t, hasPrefix(reverseHost("badexample.com"), prefix))
}

func hasPrefix(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }

func TestNormalizeDomain(t *testing.T) {
	for in, want := range map[string]string{
		"example.com":               "example.com",
		"https://Example.com/x?y=1": "example.com",
		"*.example.com":             "example.com",
		"example.com:8080":          "example.com",
		"  ":                        "",
	} {
		assert.Equal(t, want, normalizeDomain(in), in)
	}
}

func TestEscapeLike(t *testing.T) {
	assert.Equal(t, `a\%b\_c\\d`, escapeLike(`a%b_c\d`))
}
