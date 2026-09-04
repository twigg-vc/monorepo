// @ts-nocheck

import { LitElement, html, css } from 'lit';

const scriptPromises = {};

/**
 * This shit was completelly vibe coded, but it works. It's just for showing
 * internal graphs, wo it doesn't matter as long as it works.
 *
 * ## Properties
 * - DataUrl: url that will return data (see next)
 * - RefreshIntervalMs: if > 0, refetches DataUrl on this interval (paused
 *   while the tab is hidden).
 * - *data*: Array of data points to plot on the chart.
 *   Each object must include:
 *   - `Timestamp`: A string in **UTC ISO 8601** format (e.g. `"2025-11-06T21:29:00Z"`).
 *     - The `Z` suffix is required to indicate UTC time.
 *     - Offsets such as `+03:00` or missing `Z` are not allowed.
 *   - `Value`: A numeric value corresponding to that timestamp.
 *
 * A dual-handle slider above the chart restricts which slice of the points is
 * plotted, so a sub-range of the series can be inspected.
 *
 * Example:
 * ```ts
 * this.data = [
 *   { Timestamp: "2025-11-06T21:29:00Z", Value: 26 },
 *   { Timestamp: "2025-11-06T21:33:00Z", Value: 7 },
 * ];
 * this.Label = "Temperature";
 * ```
 */
export class TimeSeriesChart extends LitElement {
    static properties = {
        DataUrl: { type: String },
        Label: { type: String },
        RefreshIntervalMs: { type: Number },

        data: { type: Array },
        lastUpdatedAt: { type: String, state: true },
        // Visible window over the points, as percentages of the data range.
        rangeStartPct: { type: Number, state: true },
        rangeEndPct: { type: Number, state: true },
    };

    static styles = css`
        .chart-header {
            display: flex;
            align-items: baseline;
            gap: var(--space2);
            margin-bottom: var(--space1);
        }
        .chart-title {
            margin: 0;
        }
        .chart-updated-at {
            color: var(--color-text-muted);
            font-size: var(--space3);
        }
        .chart-container {
            position: relative;
            width: 100%;
            height: 300px;
            background-color: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius1);
            padding: var(--space2);
        }
        /*
         * Dual-handle range slider: two native range inputs stacked on top of
         * each other. The inputs are invisible except for their thumbs, and
         * only the thumbs receive pointer events, so each handle can be
         * dragged independently.
         */
        .range-slider {
            position: relative;
            height: 20px;
            margin-bottom: var(--space1);
        }
        .range-track {
            position: absolute;
            top: 50%;
            left: 0;
            right: 0;
            height: 4px;
            transform: translateY(-50%);
            background-color: var(--color-border);
            border-radius: var(--radius1);
        }
        .range-fill {
            position: absolute;
            top: 50%;
            height: 4px;
            transform: translateY(-50%);
            background-color: var(--color-primary);
            border-radius: var(--radius1);
        }
        .range-slider input[type='range'] {
            position: absolute;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            margin: 0;
            background: none;
            pointer-events: none;
            -webkit-appearance: none;
            appearance: none;
        }
        .range-slider input[type='range']::-webkit-slider-thumb {
            -webkit-appearance: none;
            appearance: none;
            pointer-events: auto;
            width: 14px;
            height: 14px;
            border: none;
            border-radius: 50%;
            background-color: var(--color-primary);
            cursor: ew-resize;
        }
        .range-slider input[type='range']::-moz-range-thumb {
            pointer-events: auto;
            width: 14px;
            height: 14px;
            border: none;
            border-radius: 50%;
            background-color: var(--color-primary);
            cursor: ew-resize;
        }
    `;

    constructor() {
        super();
        this.data = []
        this.Label = "Value"
        this.DataUrl = ""
        this.RefreshIntervalMs = 0
        this.lastUpdatedAt = ''
        this.refreshTimer = undefined
        this.rangeStartPct = 0
        this.rangeEndPct = 100
    }

    firstUpdated() {
        const canvas = this.renderRoot.querySelector('canvas');
        this.loadChartDependencies().then(() => this.initChart(canvas));
    }

    async loadChartDependencies() {
        await this.loadScript('https://cdn.jsdelivr.net/npm/chart.js');
        await this.loadScript('https://cdn.jsdelivr.net/npm/luxon');
        await this.loadScript('https://cdn.jsdelivr.net/npm/chartjs-adapter-luxon');
    }

    async loadScript(src) {
        if (scriptPromises[src]) return scriptPromises[src];

        scriptPromises[src] = new Promise((resolve, reject) => {
            if (document.querySelector(`script[src="${src}"]`)) {
                // Script exists but may not be loaded yet
                const s = document.querySelector(`script[src="${src}"]`);
                s.addEventListener('load', resolve);
                s.addEventListener('error', reject);
                return;
            }

            const s = document.createElement('script');
            s.src = src;
            s.onload = resolve;
            s.onerror = reject;
            document.head.appendChild(s);
        });

        return scriptPromises[src];
    }

    // Resolves a CSS custom property against this element, falling back when
    // the variable is not defined (e.g. in tests or missing theme).
    resolveCssColor(name, fallback) {
        const value = getComputedStyle(this).getPropertyValue(name).trim();
        if (value === '') {
            return fallback;
        }
        return value;
    }

    // Returns `color` with the given alpha. Uses the canvas to normalize any
    // CSS color into hex/rgba, since theme variables can hold any format.
    withAlpha(ctx, color, alpha) {
        ctx.fillStyle = color;
        const normalized = ctx.fillStyle;
        if (normalized.startsWith('#') && normalized.length === 7) {
            const r = parseInt(normalized.slice(1, 3), 16);
            const g = parseInt(normalized.slice(3, 5), 16);
            const b = parseInt(normalized.slice(5, 7), 16);
            return `rgba(${r}, ${g}, ${b}, ${alpha})`;
        }
        if (normalized.startsWith('rgba(')) {
            return normalized.replace(/[\d.]+\)$/, `${alpha})`);
        }
        return normalized;
    }

    // Returns the chart points restricted to the [rangeStartPct, rangeEndPct]
    // window selected on the slider.
    visibleData() {
        const points = (this.data || []).map(d => ({
            x: d.Timestamp,
            y: d.Value,
        }));
        if (this.rangeStartPct === 0 && this.rangeEndPct === 100) {
            return points;
        }
        const startIdx = Math.floor(points.length * this.rangeStartPct / 100);
        const endIdx = Math.ceil(points.length * this.rangeEndPct / 100);
        return points.slice(startIdx, endIdx);
    }

    onRangeStartInput(e) {
        var value = Number(e.target.value);
        if (value > this.rangeEndPct) {
            value = this.rangeEndPct;
            e.target.value = String(value);
        }
        this.rangeStartPct = value;
    }

    onRangeEndInput(e) {
        var value = Number(e.target.value);
        if (value < this.rangeStartPct) {
            value = this.rangeStartPct;
            e.target.value = String(value);
        }
        this.rangeEndPct = value;
    }

    initChart(canvas) {
        const ctx = canvas.getContext('2d');
        const lineColor = this.resolveCssColor('--color-primary', 'steelblue');
        const gridColor = this.withAlpha(ctx, this.resolveCssColor('--color-border', 'gray'), 0.5);
        const tickColor = this.resolveCssColor('--color-text-muted', 'gray');
        const formattedData = this.visibleData();
        this.chart = new Chart(ctx, {
            type: 'line',
            data: {
                datasets: [{
                    label: this.Label,
                    data: formattedData,
                    borderColor: lineColor,
                    backgroundColor: this.withAlpha(ctx, lineColor, 0.15),
                    fill: true,
                    borderWidth: 2,
                    tension: 0.3,
                    pointRadius: 0,
                    pointHitRadius: 8,
                    pointHoverRadius: 4,
                }],
            },
            options: {
                responsive: true,           // <-- auto resize
                maintainAspectRatio: false, // <-- allow full height
                layout: {
                    padding: 0
                },
                interaction: {
                    mode: 'index',
                    intersect: false,
                },
                scales: {
                    x: {
                        type: 'time',
                        time: {
                            unit: 'minute',
                            tooltipFormat: 'yyyy-MM-dd HH:mm:ss',
                         },
                        adapters: {
                            date: {
                                zone: 'local',
                            },
                        },
                        grid: { color: gridColor },
                        ticks: { color: tickColor, maxRotation: 0, autoSkipPadding: 16 },
                    },
                    y: {
                        beginAtZero: true,
                        grid: { color: gridColor },
                        ticks: { color: tickColor },
                    },
                },
                plugins: { legend: { display: false } },
            },
        });
    }

    connectedCallback() {
        super.connectedCallback();
        this.startRefreshTimer();
    }

    startRefreshTimer() {
        if (this.refreshTimer !== undefined) {
            window.clearInterval(this.refreshTimer);
            this.refreshTimer = undefined;
        }
        if (this.RefreshIntervalMs > 0) {
            this.refreshTimer = window.setInterval(() => {
                if (!document.hidden && this.DataUrl) {
                    this.fetchData(this.DataUrl);
                }
            }, this.RefreshIntervalMs);
        }
    }

    updated(changed) {
        const chartInputsChanged = changed.has('data')
            || changed.has('rangeStartPct')
            || changed.has('rangeEndPct');
        if (chartInputsChanged && this.chart) {
            this.chart.data.datasets[0].data = this.visibleData();
            // 'none' skips the animation, so periodic refreshes don't flicker.
            this.chart.update('none');
        }

        if (changed.has('DataUrl') && this.DataUrl) {
            this.fetchData(this.DataUrl);
        }

        if (changed.has('RefreshIntervalMs')) {
            this.startRefreshTimer();
        }
    }

    disconnectedCallback() {
        super.disconnectedCallback();
        if (this.refreshTimer !== undefined) {
            window.clearInterval(this.refreshTimer);
            this.refreshTimer = undefined;
        }
        this.chart?.destroy();
    }

    render() {
        var updatedInfo = html``;
        if (this.lastUpdatedAt !== '') {
            updatedInfo = html`<span class="chart-updated-at">updated at ${this.lastUpdatedAt}</span>`;
        }
        return html`
        <div class="chart-header">
            <h3 class="chart-title">${this.Label}</h3>
            ${updatedInfo}
        </div>
        <div class="range-slider">
            <div class="range-track"></div>
            <div class="range-fill"
                style="left: ${this.rangeStartPct}%; right: ${100 - this.rangeEndPct}%"></div>
            <input type="range" min="0" max="100" step="1"
                .value=${String(this.rangeStartPct)}
                @input=${this.onRangeStartInput}
                aria-label="Range start">
            <input type="range" min="0" max="100" step="1"
                .value=${String(this.rangeEndPct)}
                @input=${this.onRangeEndInput}
                aria-label="Range end">
        </div>
        <div class="chart-container">
            <canvas></canvas>
        </div>
    `;
    }

    async fetchData(url) {
        try {
            const res = await fetch(url);
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            this.data = await res.json(); // triggers chart update via `updated`
            this.lastUpdatedAt = new Date().toLocaleTimeString();
        } catch (err) {
            console.error("Failed to fetch chart data:", err);
        }
    }

}

customElements.define('time-series-chart', TimeSeriesChart);
