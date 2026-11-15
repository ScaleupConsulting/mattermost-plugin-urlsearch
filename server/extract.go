package main

import (
	"net/url"
	"strings"

	"github.com/mattermost/mattermost/server/public/shared/markdown"
)

// extractedLink is one http(s) link found in a message, with its host split out for domain queries.
type extractedLink struct {
	URL     string
	Host    string
	HostRev string
	Path    string
}

// extractLinks returns the distinct http(s) links in a message, in order of appearance.
// It uses Mattermost's own markdown parser, so it sees exactly what the client renders as a link:
// bare autolinks (trailing punctuation already trimmed), www. links, inline and reference links.
// Links inside code spans and code blocks are not links when rendered, so they are skipped.
func extractLinks(message string) []extractedLink {
	var links []extractedLink
	seen := map[string]bool{}
	markdown.Inspect(message, func(node any) bool {
		var dest string
		switch n := node.(type) {
		case *markdown.Autolink:
			dest = n.Destination()
		case *markdown.InlineLink:
			dest = n.Destination()
		case *markdown.ReferenceLink:
			dest = n.Destination()
		default:
			return true
		}
		if l, ok := parseLink(dest); ok && !seen[l.URL] {
			seen[l.URL] = true
			links = append(links, l)
		}
		return true
	})
	return links
}

// parseLink accepts an absolute http or https URL with a host and splits out its parts.
func parseLink(raw string) (extractedLink, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return extractedLink{}, false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	path := u.EscapedPath()
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return extractedLink{URL: raw, Host: host, HostRev: reverseHost(host), Path: path}, true
}

// reverseHost turns "www.example.com" into "com.example.www." so that a prefix match on
// "com.example." finds example.com and every subdomain of it, but not badexample.com.
func reverseHost(host string) string {
	labels := strings.Split(strings.ToLower(strings.Trim(host, ".")), ".")
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
	return strings.Join(labels, ".") + "."
}

// normalizeDomain reduces user input like "https://Example.com/x" or "*.example.com" to "example.com".
func normalizeDomain(input string) string {
	d := strings.TrimSpace(strings.ToLower(input))
	if i := strings.Index(d, "://"); i >= 0 {
		d = d[i+3:]
	}
	if i := strings.IndexAny(d, "/?#:"); i >= 0 {
		d = d[:i]
	}
	return strings.Trim(strings.TrimPrefix(d, "*."), ".")
}
