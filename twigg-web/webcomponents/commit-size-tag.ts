import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Commit } from './interfaces';
import { GetFeatureFlags } from './feature-flags';

type CommitSize = "unknown" | "XS" | "S" | "M" | "L" | "XL";

// Simple span (tag) that shows a collored commit size (XS..XL)
export class CommitSizeTag extends LitElement {
    static properties = {
        Commit: { type: Object },
    };
    constructor() {
        super();
    }
    declare Commit: Commit

    render() {
        if (!GetFeatureFlags().ShowCommitSize){
            return html``
        }

        let size = this.computeCommitSize(this.Commit)
        if (size === "unknown") {
            return html``
        }
        return html`
            <span class="pill pill--${size.toLowerCase()}" title=${this.commitSizeTooltip(size)}>${size}</span>
        `
    }

    private computeCommitSize(commit: Commit): CommitSize {
        if (!commit.HasDiffData) return "unknown"

        const lines =
            commit.DiffDataLinesCreated +
            commit.DiffDataLinesDeleted +
            commit.DiffDataLinesModified

        // deleted files are cheap to review
        const files =
            commit.DiffDataFilesCreated +
            commit.DiffDataFilesModified

        // byLines is biased towards saying the commit is large.
        const byLines = function(){
            // XS
            if (lines <= 9){
                return 0
            }
            // S
            if (lines <= 29) {
                return 1
            }
            // M
            if (lines <= 99) {
                return 2
            }
            // L
            if (lines <= 499) {
                return 3
            }
            // XL
            return 4
        }()
        // byFiles is biased towards saying the commit is small.
        const byFiles = function(){
            // XL
            if (files > 50) {
                return 4
            }
            // L
            if (files > 20) {
                return 3
            }
            // M
            if (files > 10) {
                return 2
            }
            // S
            if (files > 5) {
                return 1
            }
            // XS
            return 0
        }()

        // size is the worse of line churn and file spread
        const size = Math.max(byLines, byFiles)

        return (["XS", "S", "M", "L", "XL"] as const)[size]
    }

    private commitSizeTooltip(c: CommitSize): string {
        switch(c) {
            case "unknown": return "Unknown Size"
            case "XS": return "Extra Small"
            case "S": return "Small"
            case "M": return "Medium"
            case "L": return "Large"
            case "XL": return"Extra Large"
        }
        throw new Error("unexpected commit size: " + c)
    }

    static styles = [
        TwiggCss,
        css`
        .pill {
            font-weight: var(--weight-very-bold);
        }
        .pill--xs {
            color: var(--color-primary);
        }
        .pill--s {
            color: var(--color-primary);
        }
        .pill--m {
            color: var(--color-primary);
        }
        .pill--l {
            color: var(--color-primary);
        }
        .pill--xl {
            color: var(--color-primary);
        }
    `];
}
customElements.define('commit-size-tag', CommitSizeTag);
declare global {
    interface HTMLElementTagNameMap {
        'commit-size-tag': CommitSizeTag;
    }
}