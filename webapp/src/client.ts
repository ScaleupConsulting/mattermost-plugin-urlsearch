import manifest from 'manifest';

import {Client4} from 'mattermost-redux/client';

let basePath = '';

/**
 * Keeps only the path of the SiteURL so requests stay same-origin under any hostname.
 * @param siteURL - the server's configured SiteURL
 */
export function setSiteURL(siteURL: string | undefined) {
    basePath = siteURL ? new URL(siteURL).pathname.replace(/\/$/, '') : '';
}

export type Link = {
    post_id: string;
    channel_id: string;
    channel_name: string;
    user_id: string;
    username: string;
    create_at: number;
    url: string;
    permalink: string;
};

export type LinkPage = {links: Link[]; page: number; per_page: number; has_more: boolean};

/**
 * Calls a plugin GET route with the session's credentials, throwing the server's error text on failure.
 * @param path - path under /api/v1
 * @param params - query parameters; empty values are left out
 * @returns the parsed JSON body
 */
async function getJSON<T>(path: string, params: Record<string, string>): Promise<T> {
    const query = new URLSearchParams(Object.entries(params).filter(([, v]) => v !== '')).toString();
    const response = await fetch(`${basePath}/plugins/${manifest.id}/api/v1${path}?${query}`, Client4.getOptions({method: 'get'}));
    const body = await response.json().catch(() => null);
    if (!response.ok) {
        throw new Error(body?.error ?? `Request failed with status ${response.status}`);
    }
    return body as T;
}

export type SearchParams = {q: string; teamId: string; archived: boolean; page: number};
export type ListParams = {channelId: string; teamId: string; domain: string; archived: boolean; page: number};

/**
 * Searches links in the user's channels of a team, plus DMs and GMs, by URL fragment.
 * @param p - the fragment, current team, archived flag and zero-based page
 * @returns one page of results
 */
export function searchLinks(p: SearchParams): Promise<LinkPage> {
    return getJSON<LinkPage>('/search', {q: p.q, team_id: p.teamId, archived: String(p.archived), page: String(p.page)});
}

/**
 * Lists links in one channel, optionally only for a domain and its subdomains.
 * @param p - the channel, current team, domain, archived flag and zero-based page
 * @returns one page of results
 */
export function listLinks(p: ListParams): Promise<LinkPage> {
    return getJSON<LinkPage>('/links', {channel_id: p.channelId, team_id: p.teamId, domain: p.domain, archived: String(p.archived), page: String(p.page)});
}
