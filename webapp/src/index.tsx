import manifest from 'manifest';
import React from 'react';
import type {Store} from 'redux';

import type {Channel} from '@mattermost/types/channels';
import type {GlobalState} from '@mattermost/types/store';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import type {PluginRegistry} from 'types/mattermost-webapp';

import {setSiteURL} from './client';
import LinksRHS from './components/links_rhs';
import {SearchButton, SearchHints, SearchSuggestions} from './components/search_parts';
import {setView} from './view_state';

/**
 * Channel header icon: a chain link.
 * @returns the svg icon
 */
function LinkIcon() {
    return (
        <svg
            width='16'
            height='16'
            viewBox='0 0 24 24'
            fill='currentColor'
            aria-hidden='true'
        >
            <path d='M10.6 13.4a1 1 0 0 1 0-1.4l3.5-3.5a1 1 0 1 1 1.4 1.4L12 13.4a1 1 0 0 1-1.4 0zm-2.1 6.3a4.5 4.5 0 0 1-3.2-7.7l2.1-2.1a1 1 0 1 1 1.4 1.4l-2.1 2.1a2.5 2.5 0 0 0 3.5 3.5l2.1-2.1a1 1 0 1 1 1.4 1.4l-2.1 2.1a4.5 4.5 0 0 1-3.1 1.4zm7.4-5.3a1 1 0 0 1-.7-1.7l2.1-2.1a2.5 2.5 0 0 0-3.5-3.5l-2.1 2.1a1 1 0 1 1-1.4-1.4l2.1-2.1a4.5 4.5 0 0 1 6.4 6.4l-2.1 2.1a1 1 0 0 1-.8.2z'/>
        </svg>
    );
}

export default class Plugin {
    /**
     * Registers the Links search type, the results sidebar and the channel header link list.
     * @param registry - the webapp plugin registry
     * @param store - the webapp Redux store
     */
    public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
        setSiteURL(getConfig(store.getState()).SiteURL);

        const {showRHSPlugin} = registry.registerRightHandSidebarComponent({component: LinksRHS, title: 'Links'});

        registry.registerSearchComponents({
            buttonComponent: SearchButton,
            suggestionsComponent: SearchSuggestions,
            hintsComponent: SearchHints,
            action: (terms: string) => {
                setView({kind: 'search', query: terms.trim()});
                store.dispatch(showRHSPlugin);
            },
        });

        registry.registerChannelHeaderButtonAction({
            icon: <LinkIcon/>,
            action: (channel: Channel) => {
                const name = channel.type === 'D' || channel.type === 'G' ? 'this conversation' : `~${channel.name}`;
                setView({kind: 'channel', channelId: channel.id, channelName: name});
                store.dispatch(showRHSPlugin);
            },
            dropdownText: 'Channel links',
            tooltipText: 'Links posted in this channel',
        });
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void;
    }
}

window.registerPlugin(manifest.id, new Plugin());
