import { LitElement, html, css } from "lit";
import { TwiggCss } from "./css";
import { MinDurationTimer } from "./min-duration-timer";

declare global {
    interface HTMLElementEventMap {
        "job-log-fetched": CustomEvent<JobLogFetched>;
    }
}
export interface JobLogFetched {
    LogContent: string
}


// Component that renders a log output and a download button.
// If LogContent="", it automatically fetches the log and
// fires job-log-fetched CustomEvent<JobLogFetched>
export class JobLog extends LitElement {
    // Url do get the log
    declare GetLogUrl: string
    // Name that will be used for the file when downloaded
    declare DownloadFileName: string
    // Used when the log content is already known.
    // It can also be used to cache fetches; because the log is not
    // fetched when this property is used
    declare LogContent: string

    static properties = {
        GetLogUrl: { type: String },
        DownloadFileName: { type: String },
        isLoadingLog: { type: Boolean, state: true },
        LogContent: { type: String},
    };
    declare private isLoadingLog: boolean;

    constructor() {
        super();
        this.GetLogUrl = "NOT DEFINED";
        this.isLoadingLog = false;
        this.LogContent = "";
    }
    connectedCallback() {
        super.connectedCallback();
        this.loadLogIfLogContentIsEmpty();
    }

    render() {
        return html`
            <div class="log-container">
                <div class="log-header">
                    <a
                    href="${this.GetLogUrl}"
                    download="${this.DownloadFileName}"
                    class="download-btn"
                    >
                    <twigg-icon icon="Download">
                        Download
                    </twigg-icon>
                    </a>
                </div>

                ${this.isLoadingLog
                    ? html`<simple-loader></simple-loader>`
                    : html`
                        <pre class="log-content">${this.LogContent}</pre>
                    `}
            </div>
        `;
    }

    private async loadLogIfLogContentIsEmpty() {
        if (this.isLoadingLog || this.LogContent != ""){return}
        this.isLoadingLog = true;
        const tm = new MinDurationTimer()
        try {
            const resp = await fetch(this.GetLogUrl);
            if (!resp.ok){
                throw "Bad response status"
            }
            await tm.Wait()
            const logContent = await resp.text();
            this.LogContent = logContent
            this.dispatchEvent(new CustomEvent<JobLogFetched>('job-log-fetched', {
                detail: {
                    LogContent: logContent,
                },
                bubbles: true,
                composed: true
            }))
        } catch (e) {
            this.LogContent = "Failed to load log.";
        } finally {
            this.isLoadingLog = false;
        }
    }

    static styles = [
        TwiggCss,
        css`
        .log-container {
            padding: var(--space1);
        }
        .log-header {
            display: flex;
            justify-content: flex-end;
        }
        .log-content {
            overflow: auto;
            background: var(--color-bg);
            color: var(--color-text);
            padding: var(--space2);
            border-radius: var(--radius0);
            white-space: pre-wrap;
        }
    `];
}

customElements.define("job-log", JobLog);
declare global {
    interface HTMLElementTagNameMap {
        'job-log': JobLog;
    }
}