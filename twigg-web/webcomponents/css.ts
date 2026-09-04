import { css } from 'lit';

export const TwiggCss = css`
    /* Reset defaults */
    *, *::before, *::after {
        margin: 0;
        padding: 0;
        box-sizing: border-box;
    }

    :host {
        font-family: var(--font-family);
        line-height: var(--line-height);
    }

    h1, h2, h3, h4, h5, h6 {
        font-weight: var(--weight-bold);
        letter-spacing: -0.02em;
        line-height: 1.2;
        color: var(--color-text);
    }
    a {
        color: var(--color-text);
        text-decoration: none;
    }
    a:hover {
        text-decoration: underline;
        color: var(--color-primary-pop);
    }
    button {
        appearance: none;
        border: 1px solid var(--color-border);
        border-radius: 999px;
        padding: var(--space1) var(--space3);
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: .15s transform;
    }
    button:hover {
        transform: translateY(-1px);
    }
    button.disabled:hover  {
        cursor: not-allowed;
        transform: none;
    }
    .twigg-tag{
        border-radius: 999px;
        padding: var(--space1) var(--space2);
        display: flex;
        align-items: center;
        justify-content: center;
        gap: var(--space1);
    }

    /* === Utility classes === */
    // Doesnt appear
    .twigg-hidden{
        display: none !important;
    }
    // Moves up slightly on hover
    .twigg-lift{
        transition: .15s transform;
    }
    .twigg-lift:hover {
        transform: translateY(-2px);
        box-shadow: var(--shadow-pop);
    }
    .twigg-card {
        margin-top: var(--space4);
        background: var(--color-surface);
        border: 1px solid var(--color-border);
        padding: var(--space4);
        border-radius: var(--radius2);
        box-shadow: var(--shadow-surface);
        display: flex;
        flex-direction: column;
        gap: var(--space2);
    }
    .crumbs {
        display: flex;
        align-items: center;
        gap: var(--space1);
        padding: var(--space1) var(--space1);
    }
    #current-crumb::part(link) {
        color: var(--color-primary-pop);
        font-weight: var(--weight-bold);
    }
    .modal-backdrop {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.5);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 10000;
    }
    .modal {
        background: var(--color-surface-alt);
        padding: var(--space5p);
        border-radius: var(--fixedSpace3);
        max-width: var(--size1);
        text-align: center;
    }
    .modal-buttons {
        display: flex;
        align-items: center;
        justify-content: space-evenly;
        margin-top: var(--space4);
    }
    .twigg-scroll::-webkit-scrollbar {
        width: 8px;
    }
    .twigg-scroll::-webkit-scrollbar-thumb {
        background-color: var(--color-border);
        border-radius: 999px;
        border: 2px solid var(--color-surface);
    }
    .repo-link {
        color: inherit;
        text-decoration: none;
    }
    .repo-link:hover {
        color: inherit;
        text-decoration: none;
    }
    .repo {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: var(--space2) var(--space4);
        margin: var(--space1) 0;
        border-radius: var(--radius1);
        background: var(--color-surface);
        box-shadow: var(--shadow-surface);
        border: 1px solid var(--color-primary);
        outline: none;
    }
    .repo:focus,
    .repo:focus-within {
        outline: 2px solid var(--color-primary);
        outline-offset: 2px;
    }
    .repo-meta {
        display: flex;
        flex-direction: column;
        gap: var(--space1);
        flex: 1;
        min-width: 0; 
    }
    .repo-name {
        margin: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .repo-desc {
        margin: 0;
        opacity: 0.8;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .repo-arrow {
        font-size: var(--space5p);
        line-height: 1;
        margin-left: var(--space3);
        user-select: none;
    }
`;