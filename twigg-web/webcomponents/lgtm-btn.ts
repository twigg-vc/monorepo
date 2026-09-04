import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { GetCsrfHeaders, UrlToPostAddLgtm, UrlToPostRemoveLgtm } from './routes';
import { Commit, ServerThread, Thread } from './interfaces';
import { MinDurationTimer } from './min-duration-timer';

/**
 * LGTM button. Handles making requests to add LGTM.
 * 
 * @fires new-thread - See `new-thread` from comments.
 * 
 */
export class LgtmBtn extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        HasLgtm: { type: Boolean },
        IsLoading: { type: Boolean },
        LatestCommit: { type: Object },
    };
    constructor() {
        super();
        this.HasLgtm = false
        this.IsLoading = false
    }
    declare RepoOwnerName: string
    declare RepoName: string
    declare HasLgtm: boolean
    declare IsLoading: boolean
    declare LatestCommit: Commit

    render() {
        if (this.IsLoading){
            return html`<simple-loader></simple-loader>`
        }
        if (this.HasLgtm){
            return html`
            <button
            id="remove-lgtm-btn"
            class="checkbox-btn is-checked"
            role="checkbox"
            aria-checked="true"
            @click=${this.onBtnClick}
            title="Click to remove approval"
            >
                <span class="box" aria-hidden="true">
                    <svg class="tick" viewBox="0 0 24 24" focusable="false" aria-hidden="true">
                        <path d="M20 6 L9 17 L4 12" />
                    </svg>
                </span>
                <span class="label">LGTM</span>
            </button>
            `  
        }
        return html`
       <button
        id="lgtm-btn"
        class="checkbox-btn"
        role="checkbox"
        aria-checked="false"
        @click=${this.onBtnClick}
        title="Looks Good To Me! Add your approval to submit"
        >
            <span class="box" aria-hidden="true">
                <svg class="tick" viewBox="0 0 24 24" focusable="false" aria-hidden="true">
                    <path d="M20 6 L9 17 L4 12" />
                </svg>
            </span>
            <span class="label">LGTM</span>
        </button>
        `
    }

    private async onBtnClick(event: Event){
        const t = new MinDurationTimer()
        event.stopPropagation()
        this.IsLoading = true

        let url: string = ""
        if (!this.HasLgtm){
            url = UrlToPostAddLgtm(this.RepoOwnerName,this.RepoName,
                this.LatestCommit.L, this.LatestCommit.Version)
        }else{
            url = UrlToPostRemoveLgtm(this.RepoOwnerName,this.RepoName,
                this.LatestCommit.L)
        }
        try {
            const resp = await fetch(url, {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            if (resp.ok){
                this.HasLgtm = !this.HasLgtm
            }
            await t.Wait()

            const newThread = (await resp.json() as Thread);
            newThread.Comments = [];
            this.dispatchEvent(new CustomEvent<Thread>("new-thread", {
                detail: newThread,
                bubbles: true,
                composed: true,
            }));

        } catch (error) {
            console.log("error submitting lgtm:", error)
        }

        await t.Wait()
        this.IsLoading = false

        this.dispatchEvent(new Event('lgtm-submit', {
            bubbles: true,
            composed: true
        }));
    }

    static styles = [
        TwiggCss,
        css`
    .checkbox-btn {
      font-weight: var(--weight-bold);
      display: inline-flex;
      align-items: center;
      gap: var(--space1);
      padding: var(--space0) var(--space2);
      border-radius: var(--radius2);
      font-size: var(--space4);
      line-height: var(--line-height);
      background: var(--color-surface);
      color: var(--color-text);
      border: var(--fixedSpace0) solid var(--color-success);
      cursor: pointer;
    }
    .checkbox-btn.is-checked {
      font-weight: var(--weight-bold);
      border: var(--fixedSpace0) solid var(--color-success);
    }
    .checkbox-btn .box {
        position: relative;
        width: var(--fixedSpace4);
        height: var(--fixedSpace4);
        border-radius: var(--radius0);
        background: var(--color-surface);
        border: var(--fixedSpace0) solid var(--color-success);
        place-items: center;
        overflow: hidden;
    }
    .checkbox-btn.is-checked .box {
        background: var(--color-success);
        border-color: var(--color-success);
    }

    .checkbox-btn .tick {
        opacity: 0;                 
        display: block;             
    }
    .checkbox-btn .tick path {
        fill: none;
        stroke: var(--color-status-text);
        stroke-width: 3;
        stroke-linecap: round;
        stroke-linejoin: round;
    }
    .checkbox-btn.is-checked .tick {
        opacity: 1;
    }
    `];



}
customElements.define('lgtm-btn', LgtmBtn);
declare global {
    interface HTMLElementTagNameMap {
        'lgtm-btn': LgtmBtn;
    }
}