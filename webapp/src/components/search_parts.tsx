import React from 'react';

type SuggestionsProps = {searchTerms: string};

/**
 * Label for the "Links" search type next to Messages and Files.
 * @returns the button text
 */
export function SearchButton() {
    return <span>{'Links'}</span>;
}

/**
 * Shown under the search box while typing in Links mode, since results only appear on Enter.
 * @param props - the current search terms
 * @returns a one-line hint
 */
export function SearchSuggestions({searchTerms}: SuggestionsProps) {
    const text = searchTerms.trim() ? `Press Enter to find links containing "${searchTerms.trim()}"` : 'Type any part of a URL: a domain, a path or a document id';
    return <div style={{padding: '12px 24px', opacity: 0.72}}>{text}</div>;
}

/**
 * Hint area for Links mode; the suggestions line already explains the search.
 * @returns nothing
 */
export function SearchHints() {
    return null;
}
