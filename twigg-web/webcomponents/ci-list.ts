import { html, css, LitElement } from "lit";
import { TwiggCss } from "./css";
import { Job } from "./interfaces";
import { GetJobsAfter, UrlToGetJobLogFile } from "./routes";
import { MinDurationTimer } from './min-duration-timer';

const CiJobsPageSize = 100

export class CiList extends LitElement {
    declare RepoOwnerName: string
    declare RepoName: string
    declare CommitServerId: number
    declare CommitServerVersion: number
    declare CiStatus: "prepared" | "started";
    declare Jobs: Job[];
    declare private isLoadingMoreCiJobs: boolean;
    declare private hasMoreCiJobs: boolean;

    static properties = {
        RepoOwnerName: { type: String },
        RepoName: { type: String },
        CommitServerId: { type: Number },
        CommitServerVersion: { type: Number },
        CiStatus: { type: String },
        Jobs: { type: Array },
        isLoadingMoreCiJobs: { type: Boolean, state: true },
        hasMoreCiJobs: { type: Boolean, state: true },
    };
    constructor() {
        super();
        this.CiStatus = "prepared"
        this.Jobs = []
        this.isLoadingMoreCiJobs = false;
        this.hasMoreCiJobs = true;
    }
    render() {
        return this.renderCiTabContent();
    }
    willUpdate(changedProps) {
        if (changedProps.has('Jobs') && this.Jobs.length < CiJobsPageSize) {
            this.hasMoreCiJobs = false;
        }
    }
    renderAnalyzingJobsDiv(){
        if (this.CiStatus == "prepared") {
            return html`<div class="ci-analysis-running">Analyzing jobs for c/${this.CommitServerId}v${this.CommitServerVersion} ...</div>`;
        }
        return html``
    }
    private renderCiTabContent() {
        if (this.CiStatus == "started" && this.Jobs.length === 0) {
            return html`<div class="ci-empty">No jobs were triggered.</div>`;
        }
        return html`
        ${this.renderAnalyzingJobsDiv()}
        <div class="ci-job-list">
            ${this.Jobs.map(job => html`
            <ci-job-row
                .job=${job}
                repoOwner=${this.RepoOwnerName}
                repoName=${this.RepoName}
                .commitServerId=${this.CommitServerId}
            ></ci-job-row>
            `)}
        </div>
            <div class="view-more-btn-container">
                ${this.renderLoadMoreBtn()}
            </div>
		`;
    }

    private renderLoadMoreBtn() {
        if (!this.hasMoreCiJobs) {
            return null
        }
        if (this.isLoadingMoreCiJobs) {
            return html`<simple-loader></simple-loader>`
        }
        return html`
            <button
                id="view-more-btn"
                @click=${this.fetchMoreCiJobs}>
                View more
            </button>
        `;
    }
    private async fetchMoreCiJobs() {
        if (this.isLoadingMoreCiJobs || !this.hasMoreCiJobs) return;

        this.isLoadingMoreCiJobs = true;
        const tm = new MinDurationTimer()

        try {
            const lastJobId = this.Jobs[this.Jobs.length-1].InternalId
            const resp = await fetch(GetJobsAfter(
                this.RepoOwnerName,
                this.RepoName,
                this.CommitServerId,
                lastJobId),
                { method: 'GET' },
            );
            if (!resp.ok) {
                throw new Error(`failed to fetch jobs: ${resp.status}`)
            }
            await tm.Wait()
            const jobs: Job[] = await resp.json()
            // No jobs -> no more pages
            if (jobs.length < CiJobsPageSize) {
                this.hasMoreCiJobs = false
                this.isLoadingMoreCiJobs = false
                return
            }
            // Append
            this.Jobs = [...this.Jobs, ...jobs]
        } catch (e) {
            console.error("failed to fetch more CI jobs:", e)
        } finally {
            this.isLoadingMoreCiJobs = false
        }
    }


    static styles = [
        TwiggCss,
        css`
        .ci-empty {
            padding: var(--space4);
            color: var(--color-text-muted);
            font-style: italic;
            text-align: center;
        }
        .ci-analysis-running {
            padding: var(--space4);
            color: var(--color-text-muted);
            font-style: italic;
            text-align: center;
        }
        .ci-job-list {
            width: 100%;
            overflow-x: auto;
            -webkit-overflow-scrolling: touch;
        }
        .download-btn{
            cursor: pointer;
        }
        .view-more-btn-container{
            margin-top: var(--space1);
            display: flex;
            justify-content: center;
        }
        `
    ];

}

customElements.define("ci-list", CiList);
declare global {
    interface HTMLElementTagNameMap {
        'ci-list': CiList;
    }
}