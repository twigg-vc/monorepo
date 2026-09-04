import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { Theme, ThemeStoreSingleton } from './theme-store';

/**
 * Simple span (tag) that shows the commit status
 */
export class CommitStatus extends LitElement {
    static properties = {
        Status: { type: String },
        Link: { type: String },
        CNumber: { type: String },
        TooltipSide: { type: String },
        theme: { type: String, state: true },
    };
    constructor() {
        super();
        this.Status = "missing-lgtm"
        this.Link = ""
        this.CNumber = ""
        ThemeStoreSingleton.Init()
        this.theme = ThemeStoreSingleton.GetTheme();
        ThemeStoreSingleton.AddObserver(this)
        this.TooltipSide = "right"
    }
    declare Status:
    "has-conflict" | "will-conflict" | "missing-lgtm" | "missing-owners-approval" |
    "unresolved-comments" | "pending-parent" | "ready" | "submitted" | "WIP" | "archived"
    declare Link: ""
    declare CNumber: ""
    declare TooltipSide: "left" | "right"
    declare private theme: Theme

    render(){
        return html`
        <span class=${this.theme}>
            ${this.renderTag()}
        </span>`
    }

    renderTag() {
        switch (this.Status){
            case "has-conflict":
                return this.renderHasConflictTag();
            case "will-conflict":
                return this.renderWillConflictTag();
            case "missing-lgtm":
                return this.renderMissingLgtmTag();
            case "missing-owners-approval":
                return this.renderMissingOwnersApprovalTag();
            case "unresolved-comments":
                return this.renderUnresolvedTag();
            case "pending-parent":
                return this.renderPendingParentTag();
            case "WIP":
                return this.renderWIPTag();
            case "archived":
                return this.renderArchivedTag();
            case "submitted":
                return this.renderSubmittedTag();
            case "ready":
                return this.renderReadyTag();
            default:
                throw "unexpected status";
        }
    }

    private renderWillConflictTag(){
        return html`
            <div class="submit-conflict-container">
                <span class="pill pill--conflict">Conflict</span>
                <div class=${`submit-conflict-tooltip ${this.TooltipSide === "left"
                    ? "submit-conflict-tooltip--left"
                    : "submit-conflict-tooltip--right"
                }`}>
                    <p>The commit can't be submitted because it would cause conflicts.</p>
                    <p>To solve this problem:</p>
                    <ul>
                        <li>Pull to get the latest commits</li>
                        <li>Rebase this commit into the commit that was last submitted</li>
                        <li>Solve the conflicts and amend the commit</li>
                        <li>Push the new version of the commit</li>
                    </ul>
                </div>
            </div>
            `
    }
    private renderHasConflictTag() {
        return html`
            <div class="submit-conflict-container">
                <span class="pill pill--conflict">Conflict</span>
                <div class=${`submit-conflict-tooltip ${this.TooltipSide === "left"
                    ? "submit-conflict-tooltip--left"
                    : "submit-conflict-tooltip--right"
                }`}>
                    <p>The commit can't be submitted because it has rebase conflicts.</p>
                    <p>To solve this problem:</p>
                    <ul>
                        <li>Amend the commit to solve the conflicts</li>
                        <li>Push the new version of the commit</li>
                    </ul>
                </div>
            </div>
            `
    }
    private renderMissingLgtmTag() {
        return html`<span class="pill pill--missing-lgtm">Missing LGTM</span>`
    }
    renderMissingOwnersApprovalTag(){
        return html`<span class="pill pill--missing-lgtm">Missing Owners LGTM</span>`
    }
    private renderUnresolvedTag() {
        return html`<span class="pill pill--unresolved">Unresolved comments</span>`
    }
    private renderPendingParentTag() {
        return html`<a href="${this.Link}" class="pill pill--pending">c/${this.CNumber} must be submitted first</a>`
    }
    private renderWIPTag() {
        return html`<span class="pill pill--wip">WIP</span>`
    }
    private renderArchivedTag() {
        return html`<span class="pill pill--archived">#ARCHIVED</span>`
    }
    private renderSubmittedTag(){
        return html`<span class="pill pill--submitted">Submitted</span>`
    }
    private renderReadyTag() {
        return html`<span class="pill pill--ready">Ready to submit</span>`
    }

    OnThemeChanged(oldTheme: Theme, newTheme: Theme) {
        this.theme = newTheme
    }

    static styles = [
        TwiggCss,
        css`
        .submit-conflict-container {
            display: inline;
            position: relative;
            cursor: help;
        }
        .submit-conflict-tooltip {
            visibility: hidden;
            background-color: var(--color-surface-alt);
            box-shadow: var(--shadow-surface-alt);
            border: 1px solid #f5bcbc;
            color: var(--color-text);
            text-align: left;
            padding: 1ch 3ch;
            border-radius: 1ch;
            position: absolute;
            z-index: 9999;
            top: 100%;
            margin-top: var(--fixedSpace1);
        }

        .submit-conflict-tooltip--right {
            left: 0;
        }

        .submit-conflict-tooltip--left {
            right: 0;
        }

        .submit-conflict-container:hover .submit-conflict-tooltip,
        .submit-conflict-container:focus-within .submit-conflict-tooltip {
            visibility: visible;
            opacity: 1;
        }
             /* Prevent line breaks inside tooltip paragraphs */
        .submit-conflict-tooltip p {
            white-space: nowrap;
        }
        /* Remove extra marging from ul */
        .submit-conflict-tooltip ul {
            margin-top: 0;
        }
        .pill {
            display: inline-block;
            padding: var(--space1) var(--space2);
            border-radius: var(--radius2);
            font: 600 12px/1.1 system-ui, sans-serif;
            border: var(--fixedSpace0) solid currentColor;
            white-space: nowrap;
        }

        .light .pill--missing-lgtm { color: #a34d1b; background: #fdf0e8; border-color: #fdd393; }
        .light .pill--conflict { color: #a31b1b; background: #fde8e8; border-color: #f5bcbc; }
        .light .pill--unresolved { color: #a06800; background: #fff4d6; border-color: #f0d59a; }
        .light .pill--pending { color: #444; background: #f2f2f2; border-color: #d9d9d9; }
        .light .pill--wip { color: #5f4b00; background: #fff4b8; border-color: #f0d97a; }
        .light .pill--ready { color: #0a7a26; background: #e8f9ed; border-color: #b8efc7; }
        .light .pill--submitted { color: #1557a0; background: #e7f1fd; border-color: #bcd8fb; }
        .light .pill--archived { color: #555; background: #efefef; border-color: #ccc; }

        .pill + .pill { margin-left: var(--space2); }

        .pill--pending:hover {
            background: #eaeaea;
            border-color: #cfcfcf;
            color: #222;
            transform: translateY(-1px);
            box-shadow: 0 2px 6px rgba(0,0,0,.08);
        }

        .dark .pill--missing-lgtm { color: #ffb48a; background: #3a1f14; border-color: #a34d1b; }
        .dark .pill--conflict { color: #ff9b9b; background: #3a1414; border-color: #a31b1b; }
        .dark .pill--unresolved { color: #ffd67d; background: #3a2e14; border-color: #8a5a00; }
        .dark .pill--pending { color: #ccc; background: #2a2a2a; border-color: #444; }
        .dark .pill--wip { color: #f5e9a8; background: #3a3500; border-color: #5c5500; }
        .dark .pill--ready { color: var(--color-success); background: var(--color-surface-alt); border-color: var(--color-success); }
        .dark .pill--submitted { color: #94c1ff; background: #0f223a; border-color: #2b4f7f; }
        .dark .pill--archived { color: #bbb; background: #2a2a2a; border-color: #555; }

    `];
}
customElements.define('commit-status', CommitStatus);
declare global {
    interface HTMLElementTagNameMap {
        'commit-status': CommitStatus;
    }
}