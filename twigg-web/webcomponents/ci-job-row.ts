import { LitElement, html, css } from "lit";
import { UrlToGetJobLogFile } from "./routes";
import type { Job, JobStatus } from "./interfaces";
import { TwiggCss } from "./css";
import { JobLogFetched } from "./job-log";
import { FormatDateTime } from "./helpers";

export class CiJobRow extends LitElement {
    declare job: Job;
    declare repoOwner: string;
    declare repoName: string;
    declare commitServerId: number;

    declare private isCollapsed: boolean;
    declare private cachedLogContent: string;

    static properties = {
        job: { type: Object },
        repoOwner: { type: String },
        repoName: { type: String },
        commitServerId: { type: Number },

        isCollapsed: { type: Boolean, state: true },
        isLoadingLog: { type: Boolean, state: true },
        cachedLogContent: { type: Boolean, state: true },
    };

    constructor() {
        super();
        this.isCollapsed = true;
        this.cachedLogContent = "";
    }

    render() {
        return html`
        <div class="main">
            ${this.renderHeader()}
            ${this.renderLog()}
        </div>
        `;
    }

    private renderHeader() {
        var title = `${this.job.Name} - c/${this.job.Commit}v${this.job.CommitVersion}`
        if (this.job.RunNumber != 0){
            title += ` - Run #${this.job.RunNumber}`
        }
        return html`
        <div class="header" @click=${this.toggleIsCollapsed}>
            <status-led .Status=${this.job.Status}></status-led>
            <div class="header-rows">
                <div class="header-top-row">
                    <span class="job-path">
                        ${this.job.Path}
                    </span>
                    <span class="job-title">
                        ${title}
                    </span>
                    <span class="job-date">
                        ${FormatDateTime(this.job.CreatedTime)}
                    </span>
                </div>
                <div>
                    <span class="job-status-label">
                        ${this.job.Status}
                    </span>
                </div>
            </div>
        </div>
    `;
    }

    private renderLog() {
        if (this.isCollapsed) { return html`` }

        const downloadUrl = UrlToGetJobLogFile(
            this.repoOwner,
            this.repoName,
            this.job.Commit,
            this.job.Id
        );
        const downloadFileName = `CI_log_c${this.job.Commit}v${this.job.CommitVersion}_${this.job.Path}_${this.job.Name}_${this.job.RunNumber}`
        return html`
            <job-log
                .LogContent=${this.cachedLogContent}
                .GetLogUrl=${downloadUrl}
                .DownloadFileName=${downloadFileName}
                @job-log-fetched=${this.onJobLogFetched}
            >
            </job-log>
        `;
    }

    private async toggleIsCollapsed(e: Event) {
        e.stopPropagation();
        this.isCollapsed = !this.isCollapsed;
    }
    private onJobLogFetched(e: CustomEvent<JobLogFetched>){
        this.cachedLogContent = e.detail.LogContent
    }
    static styles = [
        TwiggCss,
        css`
        .main{
            cursor: pointer;
            padding: var(--space2);
            margin-block: var(--space2);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            transition: background-color 0.2s ease;
        }
        .main:hover {
            background: var(--color-surface-alt);
        }
        .header {
            display: flex;
            align-items: center;
            gap: var(--space2);
            padding: var(--space1) 0;
        }
        .header-rows {
            display: flex;
            flex-direction: column;
            flex: 1;
        }
        .header-top-row {
            display: flex;
            flex-wrap: wrap;
            align-items: baseline;
            gap: var(--space2);
        }
        .job-path {
            color: var(--color-text-muted);
        }
        .job-title {
            font-weight: var(--weight-semi-bold);
            color: var(--color-text);
            font-size: var(--space4m);
        }
        .job-date {
            font-size: var(--space3);
            color: var(--color-text-muted);
            opacity: var(--disable-opacity-value);
        }
        .job-status-label {
            font-size: var(--space3);
            color: var(--color-text-muted);
            text-transform: uppercase;
        }
    `];
}

customElements.define("ci-job-row", CiJobRow);
declare global {
    interface HTMLElementTagNameMap {
        'ci-job-row': CiJobRow;
    }
}