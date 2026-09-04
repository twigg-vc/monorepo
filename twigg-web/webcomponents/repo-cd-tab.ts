import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { PipelineRef } from './interfaces';
import { GetFeatureFlags } from './feature-flags';
import { PathToPipelineRefs, PathToPipelineRefsAfter } from './routes';
import { MinDurationTimer } from './min-duration-timer';

/**
* "CD" tab of the repo display
*/
export class RepoCdTab extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        isLoadingPipelineRefs: { type: Boolean, state: true },
        pipelineRefs: { type: Array, state: true },
        failedToLoadpipelineRefs: { type: Boolean, state: true },
        hasMoreRefs: { type: Boolean, state: true },
        isLoadingMoreRefs: { type: Boolean, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare private isLoadingPipelineRefs: boolean;
    declare private pipelineRefs: PipelineRef[];
    declare private failedToLoadpipelineRefs: boolean;
    declare private hasMoreRefs: boolean;
    declare private isLoadingMoreRefs: boolean;
    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.isLoadingPipelineRefs = false;
        this.pipelineRefs = [];
        this.failedToLoadpipelineRefs = false;
        this.hasMoreRefs = false;
        this.isLoadingMoreRefs = false;
    }
    render() {
        if (!GetFeatureFlags().ShowCdJobs){
            return html``
        }
        if (this.isLoadingPipelineRefs){
            return html`<simple-loader class="refs-loader"></simple-loader>`
        }
        if (this.failedToLoadpipelineRefs) {
            return html`
            <div
                class="empty-refs-msg"
                @click=${(e: Event) => { e.stopPropagation(); this.fetchPipelineRefs() }}
            >Failed to load data - click to retry</div>
            `
        }
        if (this.pipelineRefs.length == 0){
            return html`<div class="empty-refs-msg">No CD Pipelines were created yet.</div>`
        }
        return html`
            <div class="refs-list">
                ${this.pipelineRefs.map(ref => html`
                    <repo-cd-ref
                    .RepoOwnerName=${this.RepoOwnerName}
                    .RepoName=${this.RepoName}
                    .PipelineRef=${ref}>
                    </repo-cd-ref>
                `)}
            </div>
            ${this.renderLoadMoreBtn()}
        `;
    }
    private renderLoadMoreBtn(){
        if (!this.hasMoreRefs){return html``}
        if (this.isLoadingMoreRefs){return html`<simple-loader></simple-loader>`}
        return html`
            <button
            class="load-more-btn"
            @click=${this.onLoadMoreClick}
            >
            Load more
            </button>
        `
    }
    firstUpdated() {
        this.fetchPipelineRefs();
    }
    private async fetchPipelineRefs(){
        if (this.isLoadingPipelineRefs){return}
        this.isLoadingPipelineRefs = true;
        this.failedToLoadpipelineRefs = false;
        const tm = new MinDurationTimer()
        try {
            const resp = await fetch(PathToPipelineRefs(this.RepoOwnerName, this.RepoName),
                { method: 'GET' },
            );
            await tm.Wait()
            if (!resp.ok) {
                throw "Bad response"
            }
            this.pipelineRefs = await resp.json();
            if (this.pipelineRefs.length < this.getPipelineRefsPageSize){
                this.hasMoreRefs = false
            }
        } catch (error) {
            console.log("error getting refs: ", error)
            this.failedToLoadpipelineRefs = true
            return;
        } finally{
            this.isLoadingPipelineRefs = false;
        }
    }
    private async onLoadMoreClick(){
        if (this.isLoadingMoreRefs) { return }
        this.isLoadingMoreRefs = true;
        const tm = new MinDurationTimer()
        try {
            const lastRef = this.pipelineRefs[this.pipelineRefs.length-1]
            const resp = await fetch(PathToPipelineRefsAfter(this.RepoOwnerName, this.RepoName, lastRef),
                { method: 'GET' },
            );
            await tm.Wait()
            if (!resp.ok){
                throw "Bad response"
            }
            const moreRefs: PipelineRef[] = await resp.json()
            if (moreRefs.length < this.getPipelineRefsPageSize){
                this.hasMoreRefs = false
            } else{
                const newRefs = [...this.pipelineRefs, ...moreRefs]
                this.pipelineRefs = newRefs
            }
        } catch (error) {
            alert("Failed to load more data :(")
            console.log("error loading more refs: ", error)
            return;
        } finally {
            this.isLoadingMoreRefs = false;
        }
    }

    // This is the page size the backend uses
    getPipelineRefsPageSize = 20

    static styles = [
        TwiggCss,
        css`
        .refs-loader{
            padding: var(--space4);   
        }
        .empty-refs-msg {
            padding: var(--space4);
            color: var(--color-text-muted);
            font-style: italic;
            text-align: center;
        }
        .refs-list {
            display: flex;
            flex-direction: column;
            overflow: hidden;
            margin-top: var(--space3);
        }
        .load-more-btn{
            margin: auto;
        }
        `,
    ];
}
customElements.define('repo-cd-tab', RepoCdTab);
declare global {
    interface HTMLElementTagNameMap {
        'repo-cd-tab': RepoCdTab;
    }
}