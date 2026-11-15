# URL Search (com.scaleupconsulting.urlsearch)

Find links posted in Mattermost by any part of the URL, which built-in search cannot do. Requires Mattermost 12.0+ on PostgreSQL.

- `/urlsearch <part of a URL> [--archived] [--page N]` searches your channels in the current team plus your DMs and group messages.
- `/links [domain] [--team] [--archived] [--page N]` lists links in the current channel, or the whole team with `--team`. A domain also matches its subdomains.
- `/urlsearch backfill` (system admins) reindexes every existing post. The backfill also runs once on first activation.
- On desktop and web, choose **Links** in the search box, or the link button in a channel header; results open in the right-hand sidebar.

Build with `build/docker-build.sh` (needs only Docker), then upload `dist/com.scaleupconsulting.urlsearch-<version>.tar.gz` in System Console > Plugins > Plugin Management.
