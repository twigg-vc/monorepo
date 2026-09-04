import { html, css, LitElement } from 'lit';
import { TwiggCss } from './css';
import { AdminDashRequeueDeadLetter, GetCsrfHeaders, LogsPath, PprofPath, RequestCountsPath } from './routes';
import { DeadLetterItem, QueueItem, User } from './interfaces';

const LogsRefreshIntervalMs = 10_000;
const ChartsRefreshIntervalMs = 60_000;

interface LogEntry {
    Timestamp: string;
    Message: string;
    // syslog priority: 0-3 error, 4 warning, 7 debug.
    // Currently not properly populated but we'll fix soon.
    Priority: number;
}

export class AdminDash extends LitElement {
    static properties = {
        Uptime: { type: String },
        AllocMb: { type: String},
        HeapInUseMb: { type: String},
        SysMb: { type: String},
        NumGcRuns: { type: String },
        RequestByUrlPattern: { type: Object },

        NumUsers: { type: Number },
        
        linesToLog: { type: Number, state: true },
        logEntries: { type: Array, state: true },
        logsPaused: { type: Boolean, state: true },
        logsLastUpdatedAt: { type: String, state: true },

        hasRequestByUrlPattern: { type: Boolean, state: true }, 

        LatestUsers: { type: Array },

        QueueItems: { type: Array },

        DeadLetterItems: { type: Array },
    };
    constructor() {
        super();
        this.Uptime = '-1';
        this.AllocMb = "-1"
        this.HeapInUseMb = "-1"
        this.SysMb = "-1"
        this.NumGcRuns = "-1"
        this.logEntries = [];
        this.logsPaused = false;
        this.logsLastUpdatedAt = '';
        this.logsTimer = undefined;
        this.NumUsers = 0;
        this.linesToLog = 100
        this.hasRequestByUrlPattern = false;
        this.LatestUsers = [];
        this.QueueItems = [];
        this.DeadLetterItems = [];
    }
    declare Uptime: string
    declare AllocMb: string
    declare HeapInUseMb: string
    declare SysMb: string
    declare NumGcRuns: string
    declare NumUsers: number;
    declare private linesToLog: number;
    declare private logEntries: LogEntry[];
    declare private logsPaused: boolean;
    declare private logsLastUpdatedAt: string;
    declare private logsTimer: number | undefined;
    declare RequestByUrlPattern: Record<string, number>
    declare private hasRequestByUrlPattern;
    declare LatestUsers: User[];
    declare QueueItems: QueueItem[];
    declare DeadLetterItems: DeadLetterItem[];


    connectedCallback() {
        super.connectedCallback();
        this.fetchRequestCounts();
        this.fetchLogs();
        this.logsTimer = window.setInterval(() => {
            if (!this.logsPaused) {
                this.fetchLogs();
            }
        }, LogsRefreshIntervalMs);
    }

    disconnectedCallback() {
        super.disconnectedCallback();
        if (this.logsTimer !== undefined) {
            window.clearInterval(this.logsTimer);
            this.logsTimer = undefined;
        }
    }

    render() {
        return html`
            <h1>Admin dash</h1>
            <div class="side-by-side-row">
                <div class="container-with-padding">
                    ${this.renderRequestsSection()}
                </div>
                <div class="container-with-padding">
                    <h3 class="inlile-title">Uptime:</h3> ${this.Uptime} <br>
                    <h3 class="inlile-title">Memory activelly used:</h3> ${this.AllocMb} MB <br>
                    <h3 class="inlile-title">Memory allocated:</h3> ${this.HeapInUseMb} MB <br>
                    <h3 class="inlile-title">Memory obtained from OS:</h3> ${this.SysMb} MB <br>
                    <h3 class="inlile-title">Number of Garbage Collections:</h3> ${this.NumGcRuns} <br>
                </div>
            </div>
            <div class="container-with-padding">
                ${this.renderLogsSection()}
            </div>
            <div class="container-with-padding">
                <h3>Graphs</h3>
                ${this.renderGraphs()}
            </div>
            <div class="container-with-padding">
                <h3 class="inlile-title">Number of users:</h3> ${this.NumUsers} <br>
                <h3>Latest users</h3> ${this.renderLatestUsers()} <br>
                <h3>Queue Items</h3> ${this.renderQueuedItems()} <br>
                <h3>Dead Letter Items:</h3> ${this.renderDeadLetterItems()} <br>
            </div>
            <div class="container-with-padding">
                <h3 class="inlile-title">pprof:</h3> <a href=${PprofPath}>/pprof</a><br>
            </div>
        `
    }
    private renderGraphs(){
        return html`
        <div class="chart-container">
            <time-series-chart
                Label="Requests (count)"
                DataUrl="/admindash/metric/ts/requests"
                RefreshIntervalMs=${ChartsRefreshIntervalMs}
            >
            </time-series-chart>
        </div>
        <div class="chart-container">
            <time-series-chart
                Label="Latency (millisec)"
                DataUrl="/admindash/metric/ts/requests-millis"
                RefreshIntervalMs=${ChartsRefreshIntervalMs}
            >
            </time-series-chart>
        </div>
        `
    }
    private renderLatestUsers() {
        if (!this.LatestUsers || this.LatestUsers.length === 0) {
            return html`<p>No users yet.</p>`;
        }

        return html`
        <div class="table-scroll">
        <table class="users-table">
            <thead>
                <tr>
                    <th>Username</th>
                    <th>Email</th>
                    <th>Plan</th>
                    <th>Plan qty</th>
                    <th>Old CLI key?</th>
                    <th>Quota (used/total | limited)</th>
                    <th>Quota %</th>
                </tr>
            </thead>
            <tbody>
                ${this.LatestUsers.map((u) => {
                    const quotaPercent =
                    u.TotalQuota > 0
                    ? Math.round((u.QuotaUsed / u.TotalQuota) * 100)
                    : 0;
                    return html`
                        <tr>
                            <td>${u.Username}</td>
                            <td>${u.Email}</td>
                            <td>${u.PaymentPlan}</td>
                            <td>${u.PlanQuantity}</td>
                            <td>${u.HasOldCliKey ? 'Yes' : 'No'}</td>
                            <td>${u.QuotaUsed} / ${u.TotalQuota} | ${u.QuotaLimmitted}</td>
                            <td>${quotaPercent}%</td>
                        </tr>
                    `;
                })}
            </tbody>
        </table>
        </div>
    `;
    }
    private renderQueuedItems() {
        if (!this.QueueItems || this.QueueItems.length === 0) {
            return html`<p>No queued items yet.</p>`;
        }
        return html`
        <div class="table-scroll">
        <table class="queued-items-table">
            <thead>
                <tr>
                    <th>Id</th>
                    <th>Payload Type</th>
                    <th>Payload</th>
                    <th>Created At</th>
                    <th>Available At</th>
                    <th>RetryCount</th>
                </tr>
            </thead>
            <tbody>
                ${this.QueueItems.map((qi) => {
                    return html`
                        <tr>
                            <td>${qi.Id}</td>
                            <td>${qi.PayloadType}</td>
                            <td class="cell-scroll">${qi.Payload}</td>
                            <td>${qi.CreatedAt}</td>
                            <td>${qi.AvailableAt}</td>
                            <td>${qi.RetryCount}</td>
                        </tr>
                    `;
                })}
            </tbody>
        </table>
        </div>
    `;
    }
    private renderDeadLetterItems() {
        if (!this.DeadLetterItems || this.DeadLetterItems.length === 0) {
            return html`<p>No dead letter items yet.</p>`;
        }
        return html`
        <div class="table-scroll">
        <table class="queued-items-table">
            <thead>
                <tr>
                    <th>Id</th>
                    <th>Payload Type</th>
                    <th>Payload</th>
                    <th>Original Created At</th>
                    <th>Failed At</th>
                    <th>RetryCount</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody>
                ${this.DeadLetterItems.map((dli) => {
                    return html`
                        <tr>
                            <td>${dli.Id}</td>
                            <td>${dli.PayloadType}</td>
                            <td class="cell-scroll">${dli.Payload}</td>
                            <td>${dli.OriginalCreatedAt}</td>
                            <td>${dli.FailedAt}</td>
                            <td>${dli.RetryCount}</td>
                            <td>
                                <button class="requeue-btn"
                                    @click=${() => this.requeueDeadLetter(dli.Id)}
                                >
                                    Requeue
                                </button>
                            </td>
                        </tr>
                    `;
                    })}
            </tbody>
        </table>
        </div>
    `;
    }
    private async requeueDeadLetter(id: number) {
        try {
            const res = await fetch(
                AdminDashRequeueDeadLetter(id),
                { method: "POST", headers: GetCsrfHeaders() }
            );

            if (!res.ok) {
                const msg = await res.text();
                console.error("Failed to requeue dead letter:", msg);
                alert("Failed to requeue dead letter");
                return;
            }

            // Optimistic refresh
            this.DeadLetterItems = this.DeadLetterItems.filter(dli => dli.Id !== id);
            this.requestUpdate();
        } catch (err) {
            console.error("Requeue request failed:", err);
            alert("Request failed");
        }
    }
    private async fetchRequestCounts() {
        try {
            const resp = await fetch(RequestCountsPath);
            if (!resp.ok) throw new Error(`Failed to fetch metrics: ${resp.statusText}`);
            const data = await resp.json();
            const reqs = data['all-requests-by-url-pattern'] as Record<string, number>;

            this.RequestByUrlPattern = reqs;
            this.hasRequestByUrlPattern = true;
        } catch (err) {
            console.error('Error fetching request counts:', err);
            this.RequestByUrlPattern = {};
            this.hasRequestByUrlPattern = false;
        }
    }
    async fetchLogs() {
        try {
            const res = await fetch(LogsPath + "?numLines="+ String(this.linesToLog));
            const lines = await res.json(); // array of JSON objects from journalctl
            const entries: LogEntry[] = lines.map((l: any) => {
                var timestamp = 'N/A';
                if (l.__REALTIME_TIMESTAMP) {
                    timestamp = new Date(l.__REALTIME_TIMESTAMP / 1000).toLocaleString();
                }
                var priority = 6;
                const parsed = parseInt(l.PRIORITY, 10);
                if (!Number.isNaN(parsed)) {
                    priority = parsed;
                }
                return {
                    Timestamp: timestamp,
                    Message: String(l.MESSAGE ?? ''),
                    Priority: priority,
                };
            });
            // journalctl returns oldest-first; we render newest on top.
            entries.reverse();

            const container = this.renderRoot.querySelector<HTMLDivElement>('.log-container');
            var wasAtTop = true;
            if (container) {
                wasAtTop = container.scrollTop <= 8;
            }
            this.logEntries = entries;
            this.logsLastUpdatedAt = new Date().toLocaleTimeString();
            await this.updateComplete;
            if (container && wasAtTop) {
                container.scrollTop = 0;
            }
        } catch (e) {
            console.error(e);
        }
    }

    private onlinesToLogChange(e: Event) {
        const input = e.target as HTMLInputElement;
        const parsedInput = parseInt(input.value, 10);
        // Reset the input to linesToLog if input has a bad value
        if (Number.isNaN(parsedInput) || parsedInput < 1) {
            input.value = String(this.linesToLog);
            return;
        }
        this.linesToLog = parsedInput;
        this.fetchLogs();
    }

    private toggleLogsPaused() {
        this.logsPaused = !this.logsPaused;
        if (!this.logsPaused) {
            this.fetchLogs();
        }
    }

    private logLineClass(priority: number) {
        if (priority <= 3) {
            return 'log-line-error';
        } else if (priority === 4) {
            return 'log-line-warning';
        } else if (priority >= 7) {
            return 'log-line-debug';
        } else {
            return '';
        }
    }

    private renderRequestsSection() {
        if (!this.hasRequestByUrlPattern || !this.RequestByUrlPattern) {
            return html`<p>Loading request counts...</p>`;
        }
        const entries = Object.entries(this.RequestByUrlPattern);
        if (entries.length === 0) {
            return html`<p>No requests recorded yet.</p>`;
        }
        return html`
            <h3>Request count by path</h3>
            <div class="table-scroll">
                <table>
                    ${entries.map(([url, count]) => html`
                        <tr>
                            <td><code>${url}</code></td>
                            <td>${count}</td>
                        </tr>
                    `)}
                </table>
            </div>
        `;
    }
    renderLogsSection() {
        var toggleLabel = 'Pause auto-refresh';
        if (this.logsPaused) {
            toggleLabel = 'Resume auto-refresh';
        }
        var updatedInfo = html``;
        if (this.logsPaused) {
            updatedInfo = html`<span class="log-updated-at">paused</span>`;
        } else if (this.logsLastUpdatedAt !== '') {
            updatedInfo = html`<span class="log-updated-at">updated at ${this.logsLastUpdatedAt}</span>`;
        }
        return html`
        <div class="log-header">
            <h3 class="inlile-title">Logs</h3>
            ${updatedInfo}
            <label class="log-lines-label">
                Lines:
                <input class="log-lines-input" type="number" min="1" max="5000"
                    .value=${String(this.linesToLog)}
                    @change=${this.onlinesToLogChange}
                />
            </label>
            <button class="log-toggle-btn" @click=${this.toggleLogsPaused}>${toggleLabel}</button>
        </div>
        <div class="log-container">
        ${this.logEntries.map((entry) => {
            return html`
            <div class="log-line ${this.logLineClass(entry.Priority)}">
                <span class="log-timestamp-span">${entry.Timestamp}</span>
                <span class="log-msg-span">${entry.Message}</span>
            </div>`
        })}
        </div>
    `;
    }


    static styles = [
        TwiggCss,
        css`
            .inlile-title {
                display: inline;
            }
            .container-with-padding {
                padding: var(--space2);
            }
            .side-by-side-row {
                display: flex;
                flex-wrap: wrap;
                gap: var(--space2);
            }
            .side-by-side-row > .container-with-padding {
                flex: 1;
                min-width: 0;
            }
            .table-scroll {
                overflow-x: auto;
            }
            .chart-container {
                padding: var(--space5);
            }
            .users-table {
                width: 100%;
                margin-top: var(--space1);
            }
            .users-table th,
            .users-table td {
                border: 1px solid var(--color-text);
                padding: var(--space1) var(--space2);
                text-align: left;
            }
            .queued-items-table {
                width: 100%;
                margin-top: var(--space1);
            }
            .queued-items-table th,
            .queued-items-table td {
                border: 1px solid var(--color-text);
                padding: var(--space1) var(--space2);
                text-align: left;
            }
            .cell-scroll {
                max-width: 180px;
                overflow-x: auto;
                overflow-y: hidden;
                white-space: nowrap;
            }
            .log-header {
                display: flex;
                flex-wrap: wrap;
                align-items: center;
                gap: var(--space2);
                margin-bottom: var(--space1);
            }
            .log-updated-at {
                color: var(--color-text-muted);
                font-size: var(--space3);
            }
            .log-lines-label {
                margin-left: auto;
                color: var(--color-text-muted);
                font-size: var(--space3);
            }
            .log-lines-input {
                width: var(--fixedSpace8);
            }
            .log-container {
                padding: var(--space1) 0;
                border-radius: var(--radius0);
                height: var(--size0);
                min-height: var(--fixedSpace8);
                resize: vertical;
                overflow: auto;
                border: 1px solid var(--color-border);
                background: var(--color-surface-alt);
                font-family: monospace;
                font-size: var(--space3);
                line-height: var(--line-height);
            }
            .log-line {
                display: flex;
                gap: var(--space2);
                padding: var(--space0) var(--space2);
                white-space: pre-wrap;
                word-break: break-word;
            }
            .log-line:nth-child(odd) {
                background: var(--color-surface);
            }
            .log-line:hover {
                background: var(--color-border);
            }
            .log-timestamp-span {
                color: var(--color-text-muted);
                flex-shrink: 0;
            }
            .log-line-error .log-msg-span {
                color: var(--color-danger);
            }
            .log-line-warning .log-msg-span {
                color: var(--color-warning);
            }
            .log-line-debug .log-msg-span {
                opacity: var(--disable-opacity-value);
            }
            @media (max-width: 760px) {
                .side-by-side-row {
                    flex-direction: column;
                }
                .side-by-side-row > .container-with-padding {
                    flex: none;
                }
                .chart-container {
                    padding: var(--space2) 0;
                }
            }
        `
    ];
}
customElements.define('admin-dash', AdminDash);
declare global {
    interface HTMLElementTagNameMap {
        'admin-dash': AdminDash;
    }
}