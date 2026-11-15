import {useSyncExternalStore} from 'react';

/** What the right-hand sidebar shows: a URL search, or one channel's link list. */
export type View =
    | {kind: 'search'; query: string}
    | {kind: 'channel'; channelId: string; channelName: string};

let current: View | null = null;
const listeners = new Set<() => void>();

/**
 * Switches the sidebar to a new view; the search box and the header button both call this.
 * @param view - the view to show
 */
export function setView(view: View) {
    current = view;
    listeners.forEach((l) => l());
}

/**
 * Subscribes a listener to view changes, for useSyncExternalStore.
 * @param listener - called after every change
 * @returns the unsubscribe function
 */
function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

/**
 * React hook returning the current sidebar view.
 * @returns the view, or null before anything was opened
 */
export function useView(): View | null {
    return useSyncExternalStore(subscribe, () => current);
}
