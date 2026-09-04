import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { JobStatus, Pipeline, PipelineRef, PipelineStage } from './interfaces';
import { GetFeatureFlags } from './feature-flags';
import { MinDurationTimer } from './min-duration-timer';
import { GetCsrfHeaders, PathToCancelPipelineStage, PathToManualResumePipelineStage, PathToPipeline, PathToPipelineStageIsCanceled, PathToPipelineStageOutput, PathToPipelineStages } from './routes';
import { JobLogFetched } from './job-log';
import { FormatDateTime } from './helpers';

/**
* Shows a Pipeline header and all it's stages of a pipeline
*/
export class RepoCdPipeline extends LitElement {
    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        Pipeline: { type: Object },


        isRefreshingPipeline: { type: Array, state: true },
        refreshPipelineFailed: { type: Array, state: true },


        stages: { type: Array, state: true },
        stagesIsOpen: { type: Boolean, state: true },
        fetchedStages: { type: Boolean, state: true },
        fetchStagesFailed: { type: Boolean, state: true },
        isLoadingStages: { type: Boolean, state: true },

        stageIsOpen: { type: Array, state: true },
        stageIsLoading: { type: Array, state: true },
        stageLogCache: { type: Array, state: true },
    };
    declare RepoOwnerName: string;
    declare RepoName: string;
    declare Pipeline: Pipeline

    declare private isRefreshingPipeline: boolean;
    declare private refreshPipelineFailed: boolean;

    declare private stages: PipelineStage[]
    declare private stagesIsOpen: boolean
    declare private fetchedStages: boolean
    declare private fetchStagesFailed: boolean
    declare private isLoadingStages: boolean
    declare private stageIsOpen: boolean[]
    declare private stageIsLoading: boolean[]
    declare private stageLogCache: string[];
    declare private pollInterval?: number;
    constructor() {
        super();
        this.RepoOwnerName = "";
        this.RepoName = "";
        this.Pipeline = undefined;
        this.isRefreshingPipeline = false;
        this.refreshPipelineFailed = false;
        this.stages = [];
        this.stagesIsOpen = false;
        this.fetchedStages = false;
        this.fetchStagesFailed = false;
        this.isLoadingStages = false;
        this.stageIsOpen = [];
        this.stageIsLoading = [];
        this.stageLogCache = [];
        this.pollInterval = undefined
    }
    willUpdate(changedProperties) {
        if (changedProperties.has('stages')) {
            if (this.stageIsOpen.length != this.stages.length){
                this.stageIsOpen = new Array(this.stages.length).fill(false);
                this.stageIsLoading = new Array(this.stages.length).fill(false);
                this.stageLogCache = new Array(this.stages.length).fill("");
            }
        }
    }
    connectedCallback() {
        super.connectedCallback();
        this.startPolling();
    }
    disconnectedCallback() {
        super.disconnectedCallback();
        this.stopPolling();
    }

    render() {
        if (!GetFeatureFlags().ShowCdJobs) {
            return html``
        }
        return html`
            <div class="pipeline" @click=${this.toggleOpenStages}>
                ${this.renderPipelineHeader()}
                <div class="stages-container">${this.renderStages()}</div>
            </div>
        `;
    }
    private renderPipelineHeader(){
        if (this.isRefreshingPipeline){
            return html`<simple-loader></simple-loader>`
        }
        if (this.refreshPipelineFailed) {
            return html`
            <div
                class="empty-stages-msg"
                @click=${(e: Event) => { e.stopPropagation(); this.refreshPipeline() }}
            >Failed to load data - click to retry</div>
            `
        }
        return html`
        <div class="pipeline-header">
            <status-led .Status=${this.Pipeline.Status}></status-led>
            <div class="pipeline-header-rows">
                <div class="pipeline-header-top-row">
                    <span class="pipeline-title">
                        c/${this.Pipeline.Commit}v${this.Pipeline.CommitVersion} - Run #${this.Pipeline.RunNumber}
                    </span>
                    <span class="pipeline-date">
                        ${FormatDateTime(this.Pipeline.CreatedTime)}
                    </span>
                </div>
                
                <div>
                    <span class="stage-status-label">
                        ${this.Pipeline.Status}
                    </span>
                </div>
                <div>
                ${this.Pipeline.IsCreatedByUser ?
                    html`
                    <span class="pipeline-created-by">
                        Launched by ${this.Pipeline.CreatedByUsername}
                    </span>
                    ` :
                    html``
                }
                </div>
            </div>

            <div>
                ${this.renderRefreshBtn()}
            </div>

        </div>
        `
    }
    private toggleOpenStages(event){
        event.stopPropagation()
        this.stagesIsOpen = !this.stagesIsOpen
        this.fetchStagesIfNotFetched()
    }
    private renderStages(){
        if (this.refreshPipelineFailed || this.isRefreshingPipeline){return html``}
        if (!this.stagesIsOpen){return html``}
        if (this.isLoadingStages) {
            return html`<simple-loader class="stages-loader"></simple-loader>`
        }
        if (this.fetchStagesFailed){
            return html`
            <div
                class="empty-stages-msg"
                @click=${(e: Event) => { e.stopPropagation(); this.fetchStagesIfNotFetched() }}
            >Failed to load data - click to retry</div>
            `
        }
        if (this.stages.length == 0) {
            return html`<div class="empty-stages-msg">No Stages</div>`
        }
        return html`
            <div class="stages-list">
                ${this.stages.map((stage, i) => this.renderStage(i, stage))}
            </div>
        `;
    }
    private renderStage(i: number, stage: PipelineStage) {
        const getOutputUrl = PathToPipelineStageOutput(this.RepoOwnerName, this.RepoName, this.Pipeline, i)
        const logFilename = `c${this.Pipeline.Commit}v${this.Pipeline.CommitVersion}_${this.Pipeline.Name}_${stage.Name}`
        return html`
        <div class="stage-item" @click=${(e: Event) => this.handleStageClick(e, stage, i)}>
            <div class="stage-item-header">
                <status-led .Status=${stage.Status}></status-led>
                <div class="stage-info">
                    <span class="stage-name">${stage.Name}</span>
                    <span class="stage-status-label">${stage.Status}</span>
                    <span class="pipeline-date">
                        Started at: ${FormatDateTime(stage.CreatedTime)}
                    </span>
                    ${stage.IsResumedByUser ?
                    html`<span class="stage-resumed-by">Resumed by: ${stage.ResumedByUsername}</span>`
                    : html``}
                </div>
                ${this.renderManualResumeButton(i, stage)}
                ${this.renderCancelButton(i, stage)}
            </div>
            ${ this.stageIsOpen[i] ? html`
                <div class="stage-log" @click=${e => e.stopPropagation()}>
                    <job-log
                        .GetLogUrl=${getOutputUrl}
                        .DownloadFileName=${logFilename}
                        .LogContent=${this.stageLogCache[i]}
                        @job-log-fetched=${e => this.onJobLogFetched(e, i)}
                    ></job-log>
                </div>
                ` : html``}
        </div>
    `;
    }
    private renderRefreshBtn(){
        if (this.isLoadingStages || this.isRefreshingPipeline){return html``}
        return html`
            <div
                title="Refresh data"
                class="refresh-button" 
                @click=${(e: Event) => { e.stopPropagation(); this.onRefreshClicked()}}>
                <twigg-icon icon="Refresh"></twigg-icon>
            </div>
        `
    }
    private renderManualResumeButton(i: number, stage: PipelineStage){
        const postResumeStageUrl = PathToManualResumePipelineStage(this.RepoOwnerName, this.RepoName, this.Pipeline, i)
        const isWaitingManualStart = stage.Status == "waiting-manual-start"
        if (!isWaitingManualStart){
            return html``
        }
        const isLoading = this.stageIsLoading[i]
        if (isLoading){
            return html`<simple-loader></simple-loader>`
        }
        return html`
            <button
                ?disabled=${this.stageIsLoading[i]}
                class="resume-button stage-button" 
                @click=${(e: Event) => this.onResumeManualStageClicked(e, i, postResumeStageUrl)}>
                <twigg-icon icon="Play"></twigg-icon>
                <span>Resume</span>
            </button>
        `
    }
    private renderCancelButton(i: number, stage: PipelineStage) {
        const putCancelStageUrl = PathToCancelPipelineStage(this.RepoOwnerName, this.RepoName, this.Pipeline, i)
        const canCancel = stage.Status == "posted" || stage.Status == "running"
        if (!canCancel) {
            return html``
        }
        const isLoading = this.stageIsLoading[i]
        if (isLoading) {
            return html`<simple-loader></simple-loader>`
        }
        return html`
            <button
                ?disabled=${this.stageIsLoading[i]}
                class="cancel-button stage-button" 
                @click=${(e: Event) => this.onCancelStageClicked(e, i, putCancelStageUrl)}>
                <twigg-icon icon="XMark"></twigg-icon>
                <span>Cancel</span>
            </button>
        `
    }
    private handleStageClick(event: Event, stage: PipelineStage, i: number) {
        event.stopPropagation();
        const newStagesIsOpen = [...this.stageIsOpen]
        newStagesIsOpen[i] = !this.stageIsOpen[i]
        this.stageIsOpen = newStagesIsOpen
    }
    private async onResumeManualStageClicked(e: Event, i: number, url: string) {
        e.stopPropagation();
        if (this.stageIsLoading[i]){ return }
        this.toggleStageIsLoading(i)
        try {
            const resp = await fetch(url, {
                method: 'POST',
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok){
                throw "bad response"
            }
            // Trigger a pipeline refresh
            await this.refreshPipeline()
            // Trigger a stages refresh
            this.fetchedStages = false
            await this.fetchStagesIfNotFetched()

        } catch (err) {
            alert("Failed to resume stage :(")
            console.error("Failed to resume stage", err);
        }finally{
            this.toggleStageIsLoading(i)
        }
    }
    private async onCancelStageClicked(e: Event, i: number, url: string) {
        e.stopPropagation();
        if (this.stageIsLoading[i]) { return }
        this.toggleStageIsLoading(i)
        try {
            const resp = await fetch(url, {
                method: 'PUT',
                headers: GetCsrfHeaders(),
            });
            if (!resp.ok) {
                throw "bad response"
            }
            await this.refreshAndWaitUntilStageIsCanceled(i)
        } catch (err) {
            alert("Failed to cancel stage :(")
            console.error("Failed to cancel stage", err);
        } finally {
            this.toggleStageIsLoading(i)
        }
    }
    private async refreshAndWaitUntilStageIsCanceled(i: number) {
        const timeoutMs = 15000; // 15s max wait
        const intervalMs = 1000;  // check every 1sec
        const start = Date.now();

        this.isLoadingStages = true;
        while (true){
            await this.sleep(intervalMs);
            if (Date.now() - start > timeoutMs) {
                console.warn("Timeout waiting for stage to cancel");
                return;
            }
            if (await this.getStageIsCanceled(i)){
                break
            }
        }
        await this.refreshPipeline();
        this.fetchedStages = false;
        this.isLoadingStages = false;
        await this.fetchStagesIfNotFetched();
    }
    // Returns false on any error
    private async getStageIsCanceled(i: number): Promise<boolean> {
        try {
            const resp = await fetch(PathToPipelineStageIsCanceled(
                this.RepoOwnerName, this.RepoName, this.Pipeline, i),
                { method: 'GET' },
            );
            if (!resp.ok) {
                throw "bad resp status"
            }
            const isCanceledStr = await resp.text()
            return isCanceledStr == "1"
        } catch (error) {
            console.warn("getStageIsCanceled failed: ", error)
            return false;
        }
    }

    private onRefreshClicked(){
        this.clearStageLogCache();
        this.refreshRelevantData(/*force=*/true);
    }
    private toggleStageIsLoading(i: number){
        const newStagesIsLoading = [...this.stageIsLoading]
        newStagesIsLoading[i] = !this.stageIsLoading[i]
        this.stageIsLoading = newStagesIsLoading
    }

    private onJobLogFetched(e: CustomEvent<JobLogFetched>, i: number){
        const newStageLogCache = [...this.stageLogCache]
        newStageLogCache[i] = e.detail.LogContent
        this.stageLogCache = newStageLogCache
    }
    private async fetchStagesIfNotFetched() {
        if (this.isLoadingStages){return}
        if (this.fetchedStages && !this.fetchStagesFailed){return}
        this.isLoadingStages = true
        this.fetchedStages = true
        this.fetchStagesFailed = false

        const tm = new MinDurationTimer()
        try {
            const resp = await fetch(PathToPipelineStages(this.RepoOwnerName, this.RepoName, this.Pipeline),
                { method: 'GET' },
            );
            await tm.Wait()
            if (!resp.ok){
                throw "bad resp status"
            }
            this.stages = await resp.json();
        } catch (error) {
            this.fetchedStages = false
            this.fetchStagesFailed = true
            console.log("error getting stages: ", error)
            return;
        } finally {
            this.isLoadingStages = false;
        }
    }
    private async refreshPipeline(){
        if (this.isRefreshingPipeline){return}
        this.isRefreshingPipeline = true
        this.refreshPipelineFailed = false
        try{
            const tm = new MinDurationTimer()
            const resp = await fetch(PathToPipeline(this.RepoOwnerName, this.RepoName, this.Pipeline),
                { method: 'GET' },
            );
            if (!resp.ok) {
                throw "bad resp status"
            }
            await tm.Wait()
            this.Pipeline = await resp.json()
        }catch(e){
            this.refreshPipelineFailed = true
        }finally{
            this.isRefreshingPipeline = false
        }
    }
    private async refreshRelevantData(force: boolean){
        const promises: Promise<void>[] = [];

        // A pipeline is considered done (i.e. in a final stage) if its in one
        // of these stages
        const finalPipelineStatus: JobStatus[] = [
            "success", "fail", "bad-file-format","bad-file-size", "timeout", "cancel"
        ]
        const pipelineIsDone = finalPipelineStatus.includes(this.Pipeline.Status)
        if (force || !pipelineIsDone){
            promises.push(this.refreshPipeline())
        }
        // Only refresh the stages if they are open and one is not done.
        // We don't rely on the stage of the pipeline itself to decide if we
        // want to refresh the stages because there might
        // be race conditions in which the pipeline has status="success" but
        // one of the stages is still "running". Note that this doesn't happen
        // on the server itself, but just with the data inside this webcomponent
        // because the request that gets the pipeline data is not the same that
        // gets the stages. TODO: create one request that gets the pipeline
        // and the stages combined -> than this won't be necessary and we can
        // just do "update the stages and the pipeline if the pipeline is not done"
        var allStagesAreDone = true
        const finalStageStatuses: JobStatus[] = [
            "success", "fail", "timeout", "cancel",
            "too-many-jobs",
            "bad-file-format",
            "bad-file-size",
            "exceeds-plan-limits"
        ]
        for (const stage of this.stages) {
            if (force){break}
            if (!finalStageStatuses.includes(stage.Status)){
                allStagesAreDone = false
                break
            }
        }
        if (force || (this.stagesIsOpen && !allStagesAreDone)) {
            this.fetchedStages = false;
            promises.push(this.fetchStagesIfNotFetched());
        }
        await Promise.all(promises);
    }
    private startPolling() {
        const pollPeriodMs = 30000 // 30 seconds
        this.pollInterval = window.setInterval(() => {
            this.refreshRelevantData(/*force=*/false);
        }, pollPeriodMs);
    }
    private stopPolling() {
        if (this.pollInterval) {
            clearInterval(this.pollInterval);
            this.pollInterval = undefined;
        }
    }
    private clearStageLogCache(){
        this.stageLogCache = new Array(this.stages.length).fill("");
    }
    private sleep(ms: number) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    static styles = [
        TwiggCss,
        css`
        .stages-loader{
            padding: var(--space3);
        }
        .pipeline {
            cursor: pointer;
            padding: var(--space2);
            margin-block: var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            transition: background-color 0.2s ease;
        }
        .pipeline:hover {
            background: var(--color-surface-alt);
        }
        .pipeline-header {
            display: flex;
            align-items: center;
            gap: var(--space2);
            padding: var(--space1) 0;
        }
        .pipeline-header-rows {
            display: flex;
            flex-direction: column;
            flex: 1;
            min-width: 0;
        }
        .pipeline-header-top-row {
            display: flex;
            flex-wrap: wrap;
            align-items: baseline;
            gap: var(--space2);
            min-width: 0;
        }
        .pipeline-title {
            flex: 1;
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;

            font-weight: var(--weight-semi-bold);
            color: var(--color-text);
            font-size: var(--space4m);
        }
        .pipeline-date {
            flex-shrink: 0;
            font-size: var(--space3);
            color: var(--color-text-muted);
            opacity: var(--disable-opacity-value);
        }
        .pipeline-created-by{
            font-size: var(--space3);
            color: var(--color-text-muted);
        }

        .stages-list {
            display: flex;
            flex-direction: column;
            gap: var(--space2);
            padding: var(--space3);
            margin-top: var(--space2);
            background: var(--color-surface);
            border-radius: var(--radius1);
        }
        .stage-item {
            padding: var(--space2);
            border-radius: var(--radius0);
            cursor: pointer;
            transition: background-color 0.15s ease;
        }
        .stage-item-header {
            display: flex;
            align-items: center;
            gap: var(--space3);
        }
        .stage-item:hover {
            background: var(--color-surface-alt);
        }

        /* Commom style for resume/cancel btns */
        .stage-button {
            margin-left: auto; /* Pushes button to the right */
            border: none;
            border-radius: var(--radius0);
            padding: var(--fixedSpace1) var(--fixedSpace3);
            font-family: var(--font-family);
            font-weight: var(--weight-semi-bold);
            cursor: pointer;
            transition: background-color 0.2s, box-shadow 0.2s;  
        }
        .resume-button {
            background-color: var(--color-primary);
            color: var(--color-text-on-primary);
        }
        .resume-button:hover {
            background-color: var(--color-primary-pop);
            box-shadow: var(--shadow-pop);
        }
        .resume-button:active {
            opacity: var(--disable-opacity-value);
        }
        .cancel-button{
            background-color: var(--color-soft-warning);
            color: var(--color-status-text);
        }
        .cancel-button:hover {
            background-color: var(--color-warning);
            box-shadow: var(--shadow-pop-yellow);
        }
        .cancel-button:active {
            opacity: var(--disable-opacity-value);
        }

        .refresh-button{
            color: var(--color-text);
            border: none;
            border-radius: var(--radius0);
            padding: var(--fixedSpace1) var(--fixedSpace1);
            cursor: pointer;
            font-size: var(--space4m);
            transition: transform 0.2s;
        }
        .refresh-button:hover{
            transform: translateY(-2px);
        }

        .stage-info {
            display: flex;
            flex-direction: column;
        }
        .stage-name {
            font-weight: var(--weight-semi-bold);
            color: var(--color-text);
            font-size: var(--space4m);
        }
        .stage-status-label {
            font-size: var(--space3);
            color: var(--color-text-muted);
            text-transform: uppercase;
        }
        .stage-resumed-by{
            font-size: var(--space3);
            color: var(--color-text-muted);
        }
        .empty-stages-msg {
            padding: var(--space4);
            color: var(--color-text-muted);
            text-align: center;
            font-style: italic;
        }
        .stages-loader {
            padding: var(--space3);
        }
        `,
    ];
}
customElements.define('repo-cd-pipeline', RepoCdPipeline);
declare global {
    interface HTMLElementTagNameMap {
        'repo-cd-pipeline': RepoCdPipeline;
    }
}