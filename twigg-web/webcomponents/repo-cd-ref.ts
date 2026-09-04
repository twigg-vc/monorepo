import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { PipelineRef, Pipeline, Commit } from './interfaces';
import { GetFeatureFlags } from './feature-flags';
import { GetCsrfHeaders, PathToManuallyLaunchPipeline, PathToPipelines, PathToPipelinesAfter } from './routes';
import { MinDurationTimer } from './min-duration-timer';

/**
* Shows all the CD pipelines executions given a ref (i.e. its name and name)
*/
export class RepoCdRef extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        PipelineRef: { type: Object },

        pipelines: { type: Array, state: true },
        pipelinesIsOpen: { type: Boolean, state: true },
        fetchedPipelines: { type: Boolean, state: true },
        fetchPipelinesFailed: { type: Boolean, state: true },
        isLoadingPipelines: { type: Boolean, state: true },
        hasMorePipelines: { type: Boolean, state: true },
        isLoadingMorePipelines: { type: Boolean, state: true },
        isLaunchingNew: { type: Boolean, state: true },

        isModalOpen: { type: Boolean, state: true },
        commitNumber: { type: Number, state: true },
        commitVersion: { type: Number, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare PipelineRef: PipelineRef;
    declare private pipelines: Pipeline[];
    declare private pipelinesIsOpen: boolean;
    declare private fetchedPipelines: boolean;
    declare private fetchPipelinesFailed: boolean;
    declare private isLoadingPipelines: boolean;
    declare private hasMorePipelines: boolean;
    declare private isLoadingMorePipelines: boolean;
    declare private isLaunchingNew: boolean;
    declare private isModalOpen: boolean;
    declare private selectedCommit: Commit | null;
    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.PipelineRef = undefined;
        this.pipelines = [];
        this.pipelinesIsOpen = false;
        this.fetchedPipelines = false;
        this.fetchPipelinesFailed = false;
        this.isLoadingPipelines = false;
        this.hasMorePipelines = true;
        this.isLoadingMorePipelines = false;
        this.isLaunchingNew = false;
        this.isModalOpen = false;
        this.selectedCommit = null;
    }
    willUpdate(changedProps) {
        if (changedProps.has('pipelines') && changedProps.get('pipelines') != undefined) {
            const oldLen = changedProps.get('pipelines').length
            const newLen = this.pipelines.length
            if (newLen-oldLen < this.getMorePipelinesPageSize) {
                this.hasMorePipelines = false
            }
        }
    }
    render() {
        if (!GetFeatureFlags().ShowCdJobs) {
            return html``
        }
        return html`
            <div class="pipeline-ref" @click=${this.toggleOpenPipelines}>
                <div class="pipeline-ref-header">
                    <span class="pipeline-path">${this.PipelineRef.Path}</span>
                    <span class="pipeline-name">${this.PipelineRef.Name}</span>
                    <button ?disabled=${this.isModalOpen} class="launch-btn" @click=${this.onLaunchClick}>Launch</button>
                </div>
                <div class="pipelines-container">${this.renderPipelines()}</div>
            </div>
            ${this.renderLaunchModal()}
        `;
    }
    private toggleOpenPipelines(){
        this.pipelinesIsOpen = !this.pipelinesIsOpen
        this.fetchPipelinesIfNotFetched()
    }
    private renderPipelines(){
        if (!this.pipelinesIsOpen){return html``}
        if (this.isLoadingPipelines) {
            return html`<simple-loader class="refs-loader"></simple-loader>`
        }
        if (this.fetchPipelinesFailed){
            return html`
            <div
                class="pipelines-msg"
                @click=${(e: Event) => { e.stopPropagation(); this.fetchPipelinesIfNotFetched() }}
            >Failed to load data - click to retry</div>
            `
        }
        if (this.pipelines.length == 0) {
            return html`<div class="pipelines-msg">No CD Pipelines started yet.</div>`
        }
        return html`
        ${this.pipelines.map(pipeline => html`
            <repo-cd-pipeline
            .RepoOwnerName=${this.RepoOwnerName}
            .RepoName=${this.RepoName}
            .Pipeline=${pipeline}>
            </repo-cd-pipeline>
        `)}
        ${this.renderLoadMoreBtn()}
        `
    }
    private renderLoadMoreBtn(){
        if (!this.hasMorePipelines){return html``}
        if (this.isLoadingMorePipelines){return html`<simple-loader></simple-loader>`}
        return html`
            <button
            class="load-more-btn"
            @click=${this.onLoadMoreClick}
            >
            Load more
            </button>
        `
    }
    private async onLoadMoreClick(e: Event){
        e.stopPropagation()
        if (this.isLoadingMorePipelines){return}
        this.isLoadingMorePipelines = true
        const lastPipeline = this.pipelines[this.pipelines.length-1]
        try {
            const tm = new MinDurationTimer()
            const resp = await fetch(PathToPipelinesAfter(this.RepoOwnerName, this.RepoName, lastPipeline),
                { method: 'GET' },
            );
            await tm.Wait()
            if (!resp.ok) {
                throw "Bad resp"
            }
            const morePipelines: Pipeline[] = await resp.json();
            const newPipelines = [...this.pipelines, ...morePipelines]
            this.pipelines = newPipelines;
        } catch (error) {
            alert("failed to get more pipelines :(")
            console.log("failed to get more pipelines: ", error)
            return;
        } finally {
            this.isLoadingMorePipelines = false;
        }
    }
    private async fetchPipelinesIfNotFetched() {
        if (this.fetchedPipelines || this.isLoadingPipelines){return}
        this.fetchedPipelines = true
        this.isLoadingPipelines = true
        this.fetchPipelinesFailed = false
        const tm = new MinDurationTimer()
        try {
            const resp = await fetch(PathToPipelines(this.RepoOwnerName, this.RepoName, this.PipelineRef),
                { method: 'GET' },
            );
            await tm.Wait()
            if (!resp.ok){
                throw "Bad resp"
            }
            this.pipelines = await resp.json();
        } catch (error) {
            this.fetchedPipelines = false
            this.fetchPipelinesFailed = true;
            console.log("error getting refs: ", error)
            return;
        } finally {
            this.isLoadingPipelines = false;
        }
    }
    private async launchNewPipeline(c: Commit){
        if (this.isLaunchingNew){return}
        this.isLaunchingNew = true
        try{
            const tm = new MinDurationTimer()
            const resp = await fetch(
                    PathToManuallyLaunchPipeline(this.RepoOwnerName, this.RepoName,
                        this.PipelineRef, c.L, c.Version), {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            await tm.Wait()
            if (!resp.ok) {
                throw "Bad resp"
            }
            this.closeModal()
            this.fetchedPipelines = false
            this.fetchPipelinesIfNotFetched()
        } catch(e){
            alert("Failed to launch :(")
        }finally{
            this.isLaunchingNew = false
        }
    }
    private onLaunchClick(e: Event) {
        e.stopPropagation();
        this.isModalOpen = true;
    }

    private closeModal() {
        this.isModalOpen = false;
        this.selectedCommit = null;
    }

    private async onLaunchPipelineClicked(e: Event) {
        e.stopPropagation();
        if (this.selectedCommit == null) { 
            alert("commit not selected")
            return 
        }
        await this.launchNewPipeline(this.selectedCommit)
    }

    private renderLaunchModal() {
        if (!this.isModalOpen) {
            return html``
        }
        return html`
        <div class="modal-backdrop" @click=${this.closeModal}>
            <div id="launch-pipeline-modal" class="modal" @click=${(e: Event) => e.stopPropagation()}>
                <button class="modal-close" ?disabled=${this.isLaunchingNew} @click=${this.closeModal}>✕</button>
                <h3>Launch Pipeline</h3>

                <div id="commit-selector-container">
                    <commit-selector
                    RepoOwnerName=${this.RepoOwnerName}
                    RepoName=${this.RepoName}
                    @selection-changed=${(e: CustomEvent<Commit|null>) =>this.onCommitSelected(e)}
                    >
                    </commit-selector>
                </div>

                <div id="launch-btn-container">
                    <button id="launch-btn" ?disabled=${this.isLaunchingNew} @click=${this.onLaunchPipelineClicked}>
                        ${this.isLaunchingNew ? html`<simple-loader></simple-loader>` : 'Launch'}
                    </button>
                </div>

            </div>
        </div>
    `;
    }
    private onCommitSelected(e: CustomEvent<Commit | null>){
        this.selectedCommit = e.detail
    }

    // This is the page size the backend uses
    getMorePipelinesPageSize = 10

    static styles = [
        TwiggCss,
        css`
        #launch-pipeline-modal{
            max-width: var(--size3);
            width: min(var(--size3), calc(100vw - var(--space3)));
            text-align: start;
            position: relative;
        }
        .modal-close {
            position: absolute;
            top: var(--space3);
            right: var(--space3);
            background: none;
            border: none;
            cursor: pointer;
            color: var(--color-text-muted);
            padding: var(--space1);
            border-radius: var(--radius0);
        }
        #commit-selector-container{
            min-width: 0;
            padding-top: var(--space1);
        }
        #launch-btn-container{
            padding: var(--space1) 0;
            display: flex;
            justify-content: center;
        }
        #launch-btn{
            background-color: var(--color-primary);
            color: var(--color-text-on-primary);
        }
        .pipeline-ref {
            padding: var(--space3) var(--space4);
            margin-bottom: var(--space3);
            border-bottom: 1px solid var(--color-border);
            background: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
        }
        .pipeline-name {
            font-weight: var(--weight-semi-bold);
            color: var(--color-text);
        }
        .pipeline-path {
            color: var(--color-text-muted);
        }
        .pipelines-msg {
            padding: var(--space4);
            color: var(--color-text-muted);
            text-align: center;
            font-style: italic;
        }
        .load-more-btn{
            margin: auto;
        }
        .pipeline-ref-header {
            display: flex;
            align-items: center;
            gap: var(--space3);
        }
        .launch-btn {
            margin-left: auto;
            background-color: var(--color-surface);
            color: var(--color-text);
        }
        `,
    ];
}
customElements.define('repo-cd-ref', RepoCdRef);
declare global {
    interface HTMLElementTagNameMap {
        'repo-cd-ref': RepoCdRef;
    }
}