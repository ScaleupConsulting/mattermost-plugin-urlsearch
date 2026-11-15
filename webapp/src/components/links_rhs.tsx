import React, {useCallback, useEffect, useState} from 'react';
import {useSelector} from 'react-redux';

import type {GlobalState} from '@mattermost/types/store';

import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {listLinks, searchLinks} from '../client';
import type {Link, LinkPage} from '../client';
import {useView} from '../view_state';
import type {View} from '../view_state';

const row: React.CSSProperties = {padding: '8px 0', borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)'};
const meta: React.CSSProperties = {fontSize: 12, opacity: 0.64};
const urlStyle: React.CSSProperties = {wordBreak: 'break-all'};
const errorStyle: React.CSSProperties = {color: 'var(--error-text, #d24b4e)'};

/**
 * Fetches one page for a view with the current filters.
 * @param view - search or channel view
 * @param teamId - the current team, used for scope and DM permalinks
 * @param domain - domain filter for the channel view
 * @param archived - whether to include archived channels
 * @param page - zero-based page
 * @returns the page
 */
function fetchPage(view: View, teamId: string, domain: string, archived: boolean, page: number): Promise<LinkPage> {
    if (view.kind === 'search') {
        return searchLinks({q: view.query, teamId, archived, page});
    }
    return listLinks({channelId: view.channelId, teamId, domain, archived, page});
}

/**
 * Right-hand sidebar listing search results or a channel's links, newest first, with paging.
 * @returns the sidebar body
 */
export default function LinksRHS() {
    const view = useView();
    const teamId = useSelector((state: GlobalState) => getCurrentTeamId(state));
    const [domain, setDomain] = useState('');
    const [appliedDomain, setAppliedDomain] = useState('');
    const [archived, setArchived] = useState(false);
    const [links, setLinks] = useState<Link[]>([]);
    const [page, setPage] = useState(0);
    const [hasMore, setHasMore] = useState(false);
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);

    const load = useCallback(async (nextPage: number) => {
        if (!view) {
            return;
        }
        setBusy(true);
        setError('');
        try {
            const result = await fetchPage(view, teamId, appliedDomain, archived, nextPage);
            setLinks((prev) => (nextPage === 0 ? result.links : [...prev, ...result.links]));
            setPage(nextPage);
            setHasMore(result.has_more);
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setBusy(false);
        }
    }, [view, teamId, appliedDomain, archived]);

    useEffect(() => {
        load(0);
    }, [load]);

    useEffect(() => {
        setDomain('');
        setAppliedDomain('');
    }, [view]);

    if (!view) {
        return <div style={{padding: 16}}>{'Choose Links in the search box, or the link button in a channel header.'}</div>;
    }

    const heading = view.kind === 'search' ? `Links containing "${view.query}"` : `Links in ${view.channelName}`;

    return (
        <div style={{padding: '8px 16px', overflowY: 'auto', height: '100%'}}>
            <h4 style={{margin: '8px 0'}}>{heading}</h4>
            {view.kind === 'channel' && (
                <form
                    onSubmit={(e) => {
                        e.preventDefault();
                        setAppliedDomain(domain.trim());
                    }}
                    style={{display: 'flex', gap: 8, marginBottom: 8}}
                >
                    <input
                        className='form-control'
                        placeholder='Filter by domain, e.g. github.com'
                        value={domain}
                        onChange={(e) => setDomain(e.target.value)}
                    />
                    <button
                        className='btn btn-primary'
                        type='submit'
                    >
                        {'Filter'}
                    </button>
                </form>
            )}
            <label style={{fontWeight: 'normal', display: 'block', marginBottom: 8}}>
                <input
                    type='checkbox'
                    checked={archived}
                    onChange={(e) => setArchived(e.target.checked)}
                />
                {' Include archived channels'}
            </label>
            {error && <div style={errorStyle}>{error}</div>}
            {!busy && !error && links.length === 0 && <div>{'No links found.'}</div>}
            {links.map((l) => (
                <div
                    key={`${l.post_id}:${l.url}`}
                    style={row}
                >
                    <a
                        href={l.url}
                        onClick={(e) => {
                            e.preventDefault();
                            window.open(l.url, '_blank', 'noopener,noreferrer');
                        }}
                        style={urlStyle}
                    >
                        {l.url}
                    </a>
                    <div style={meta}>
                        {`${l.channel_name} by @${l.username}, ${new Date(l.create_at).toLocaleString()} `}
                        <a href={l.permalink}>{'Jump'}</a>
                    </div>
                </div>
            ))}
            {hasMore && (
                <button
                    className='btn btn-tertiary'
                    disabled={busy}
                    onClick={() => load(page + 1)}
                    style={{margin: '12px 0'}}
                >
                    {busy ? 'Loading...' : 'Load more'}
                </button>
            )}
        </div>
    );
}
