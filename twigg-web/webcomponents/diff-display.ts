import { html, css, LitElement, PropertyValues } from 'lit';
import { unsafeHTML } from "lit/directives/unsafe-html.js";
import { HighlightJsStyle } from './highlight-js-stylesheet';
import { HighlightCharsAndSanitize, HighlightLineCharsAndSanitize } from './char-diff'
import { LeftAndRightLines, NewEmptyLinesSet, ParseUnifiedDiff } from './unified-diff'
import { AlignmentPoint, DiffAlgorithm, LayoutRows } from './align-layout'
import hljs from 'highlight.js/lib/core';
import DOMPurify from 'dompurify';

import go from 'highlight.js/lib/languages/go';
hljs.registerLanguage('go', go);

import javascript from 'highlight.js/lib/languages/javascript';
hljs.registerLanguage('javascript', javascript);

import typescript from 'highlight.js/lib/languages/typescript';
hljs.registerLanguage('typescript', typescript);

import markdown from 'highlight.js/lib/languages/markdown';
hljs.registerLanguage('markdown', markdown);

import json from 'highlight.js/lib/languages/json';
hljs.registerLanguage('json', json);

import xml from 'highlight.js/lib/languages/xml';
import { TwiggCss } from './css';
import { Theme, ThemeStoreSingleton } from './theme-store';
hljs.registerLanguage('xml', xml);

import cssLang from 'highlight.js/lib/languages/css';
hljs.registerLanguage('css', cssLang);

import scss from 'highlight.js/lib/languages/scss';
hljs.registerLanguage('scss', scss);

import yaml from 'highlight.js/lib/languages/yaml';
hljs.registerLanguage('yaml', yaml);

import less from 'highlight.js/lib/languages/less';
import { AlignedRow } from './differs';
hljs.registerLanguage('less', less);

// Set to true to show the button that asks for a slot on a line
const requestLineSlotBtnIsEnabled = true

// Class put on the characters that changed within a line
const charChangedClass = "char-changed"

export interface RequestLineSlot {
    LeftLine: number | ""
    RightLine: number | ""
}
declare global {
    interface HTMLElementEventMap {
        "request-line-slot": CustomEvent<RequestLineSlot>;
    }
}

/**
 * Element that displays a unified diff.
 * @fires request-line-slot - See `request-line-slot` above; fired when the
 * user asks for a slot on a line.
 * @slot left - Elements that are shown on the left
 * @slot right - Elements shown on the right
 * @slot left-line-N - Elements shown under line N of the left file,
 * if LeftSlotLines property contains N. Use DiffDisplay.LeftLineSlotName to build the slot name.
 * @slot right-line-N - Same as left-line-N, for the right file, if
 * RightSlotLines property contains N. Use DiffDisplay.RightLineSlotName to build the name.
 */
export class DiffDisplay extends LitElement {
    // Name of the slot rendered under the given line of the left file
    // if LeftSlotLines property contains `line`
    public static LeftLineSlotName(line: number): string {
        return `left-line-${line}`
    }
    // Name of the slot rendered under the given line of the right file
    // if RightSlotLines property contains `line`
    public static RightLineSlotName(line: number): string {
        return `right-line-${line}`
    }
    // Name of the file on the left
    declare public leftFilename: string
    // Name of the file on the right
    declare public rightFilename: string
    // Unified diff of the files
    declare public unifiedDiff: string;
    // Lines of the left file under which a `left-line-N` slot is rendered.
    declare public LeftSlotLines: number[]
    // Lines of the right file under which a `right-line-N` slot is rendered.
    declare public RightSlotLines: number[]
    static properties = {
        leftFilename: { type: String },
        rightFilename: { type: String },
        unifiedDiff: { type: String },
        LeftSlotLines: { type: Array },
        RightSlotLines: { type: Array },
        DiffAlgorithm: { type: String },

        blocks: { type: Array, state: true },
        alignPoints: { type: Array, state: true },
        pendingAlignmentPick: { type: Object, state: true },

        theme: { type: String, state: true },
    };
    constructor() {
        super();
        this.DiffAlgorithm = "myers"
        this.unifiedDiff = "";
        this.leftFilename = ""
        this.rightFilename = ""
        this.LeftSlotLines = []
        this.RightSlotLines = []
        this.blocks = [];
        this.parsedUnifiedDiffLines = { Left: NewEmptyLinesSet(), Right: NewEmptyLinesSet() };
        this.leftLang = "";
        this.rightLang = "";
        this.alignPoints = [];
        this.pendingAlignmentPick = undefined;
        this.nContextLines = 5;
        ThemeStoreSingleton.Init()
        this.theme = ThemeStoreSingleton.GetTheme();
        ThemeStoreSingleton.AddObserver(this)
    }
    declare public DiffAlgorithm: DiffAlgorithm
    declare private nContextLines: number
    declare private theme: Theme

    // Pairs of lines to be put on the same row.
    declare private alignPoints: AlignmentPoint[]
    // The line picked on one side, waiting for its pair on the other.
    declare private pendingAlignmentPick: PickedAlignmentLine | undefined

    // All following properties are derived from the fields above.
    declare private parsedUnifiedDiffLines: LeftAndRightLines
    declare private leftLang: string
    declare private rightLang: string
    declare private blocks: Block[]

    static styles = [
        TwiggCss,
        css`
        table.diff-table {
            width: 100%;
            table-layout: fixed;
            border-collapse: collapse;
            font-family: monospace;
        }

        td.line-num {
            color: var(--diff-line-number-color);
            user-select: none;
            font-size: var(--space3);
            padding: 0;
            margin: 0;
            padding-right: var(--space0);
        }
        .line-num-inner {
            display: flex;
            justify-content: right; /* horizontal */
            align-items: center;     /* vertical */
        }

        td.line-content {
            user-select: text;
            white-space: pre-wrap;
            word-break: break-word;
            background-color: var(--color-surface-alt);
            color: var(--diff-line-text-color);
        }

        td.show-more{
            text-align: center;
            padding: var(--space2);
        }
        .show-more-btn-wrapper{
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100%;
        }
        .show-more-btn{
            font-size: var(--space4m);
            background-color: var(--color-surface);
            color: var(--color-primary);
        }

        pre, code {
            padding: 0 !important;
            margin: 0;
            white-space: pre-wrap;   /* wrap long lines */
            word-break: break-word;  /* break very long words if needed */
        }

        /* Background for deletions and additions */
        .deleted-line {
            background-color:  var(--diff-line-removed-color-bg) !important;
        }
        .added-line {
            background-color: var(--diff-line-added-color-bg) !important;
        }
        .deleted-line .char-changed {
            background-color: var(--diff-char-removed-color-bg);
        }
        .added-line .char-changed {
            background-color: var(--diff-char-added-color-bg);
        }

        table.selecting-left td.line-content.right,
        table.selecting-right td.line-content.left {
            user-select: none;
        }

        .container {
            display: flex;
        }
        .container > div {
            flex: 1; /* each child takes equal space */
            min-width: 0;
            max-width: 100%;
        }
        .pick-alignment-btn,
        .request-line-slot-btn {
            visibility: hidden;
            font-family: monospace;
            font-size: var(--space3);
            line-height: 1;
            background-color: var(--color-surface);
            color: var(--color-primary);
            border: 1px solid var(--color-border);
            border-radius: var(--radius0);
            padding: 0 var(--space0);
            margin-right: var(--space0);
            cursor: pointer;
        }
        tr:hover .pick-alignment-btn,
        tr:hover .request-line-slot-btn {
            visibility: visible;
        }
        .pick-alignment-btn:hover,
        .request-line-slot-btn:hover {
            color: var(--color-primary-pop);
            background-color: var(--color-surface-alt);
        }
        .pick-alignment-btn.picked {
            visibility: visible;
            background-color: var(--color-primary);
            color: var(--color-text-on-primary);
        }
        .pick-alignment-btn.awaiting {
            visibility: visible;
            border-color: var(--color-primary);
        }
        .pick-alignment-btn.aligned {
            visibility: visible;
            border-color: var(--color-primary);
            color: var(--color-primary-pop);
        }

        tr.slot-row > td.slot-cell {
            vertical-align: top;
            padding: var(--space1);
            background-color: var(--color-bg);
            border-top: 1px solid var(--color-border);
            border-bottom: 1px solid var(--color-border);
        }

        #no-content-container{
            display: flex;
            justify-content:center;
            margin: var(--space2);
            color: var(--color-text-muted);
        }
    `,
    HighlightJsStyle,
    ];


    render() {
        if (this.blocks.length === 0) {
            return html`
            <div id="no-content-container">
                <span>No content</span>
            </div>
            ${this.renderSlotsContainer()}
            `;
        }
        const lastBlock = this.blocks[this.blocks.length - 1];
        const nDigits = getNumOfDigitsOfMaxRow(lastBlock.GetAll());

        return html`
        <table class="diff-table ${this.theme}" @mousedown=${this.onTableMouseDown}>
            <colgroup>
                <col style="width:${nDigits + 1}ch;">
                <col>
                <col style="width:${nDigits + 1}ch;">
                <col>
            </colgroup>
            <tbody>
                ${this.blocks.map((block, idx) => this.renderBlock(block, idx))}
            </tbody>
        </table>

        ${this.renderSlotsContainer()}
        `;
    }
    private renderSlotsContainer(){
        return html`
        <div class="container">
            <div>
                <slot name="left"></slot>
            </div>
            <div>
                <slot name="right"></slot>
            </div>
        </div>
        `
    }

    // willUpdate refreshes the derived fields, before the render that reads
    // them. Each one is only redone when something it is built from changed.
    willUpdate(changed: PropertyValues) {
        var mustUpdateBlocks = false;
        if (changed.has("unifiedDiff")) {
            this.parsedUnifiedDiffLines = ParseUnifiedDiff(this.unifiedDiff);
            this.alignPoints = [];
            this.pendingAlignmentPick = undefined;
            mustUpdateBlocks = true;
        }
        if (changed.has("leftFilename")) {
            this.leftLang = DiffDisplay.getFileLanguage(this.leftFilename);
            mustUpdateBlocks = true;
        }
        if (changed.has("rightFilename")) {
            this.rightLang = DiffDisplay.getFileLanguage(this.rightFilename);
            mustUpdateBlocks = true;
        }
        if (changed.has("alignPoints")) {
            mustUpdateBlocks = true;
        }
        if (changed.has("DiffAlgorithm")) {
            mustUpdateBlocks = true;
        }
        if (mustUpdateBlocks) {
            const rows = LayoutRows(
                this.parsedUnifiedDiffLines.Left.Raw,
                this.parsedUnifiedDiffLines.Right.Raw,
                this.alignPoints, this.DiffAlgorithm);
            const lines: DiffLine[] = [];
            for (const row of rows) {
                lines.push(this.getDiffLine(row, this.parsedUnifiedDiffLines,
                    this.leftLang, this.rightLang));
            }
            this.blocks = this.getBlocks(lines);
        }
        if (mustUpdateBlocks || changed.has("LeftSlotLines")
            || changed.has("RightSlotLines")) {
            this.showBlocksWithLineSlot()
        }
    }

    // showBlocksWithLineSlot un-hides the blocks holding a line that has a slot
    private showBlocksWithLineSlot() {
        for (const b of this.blocks) {
            if (!b.IsHidden()) {
                continue
            }
            if (this.hasLineSlot(b)) {
                b.ForceShow()
            }
        }
    }

    private hasLineSlot(b: Block): boolean {
        for (const l of b.GetAll()) {
            if (l.leftNum !== "" && this.LeftSlotLines.includes(l.leftNum)) {
                return true
            }
            if (l.rightNum !== "" && this.RightSlotLines.includes(l.rightNum)) {
                return true
            }
        }
        return false
    }

    // pickLineAlignment pairs the row up with the one already picked on the
    // other side. Picking an already aligned row drops that alignment.
    private pickLineAlignment(side: Side, rowNumber: number) {
        if (this.dropLineAlignmentAt(side, rowNumber)) {
            return;
        }
        if (this.pendingAlignmentPick === undefined) {
            this.pendingAlignmentPick = { Side: side, AlignmentRowNumber: rowNumber };
            return;
        }
        if (this.pendingAlignmentPick.Side === side) {
            if (this.pendingAlignmentPick.AlignmentRowNumber === rowNumber) {
                this.pendingAlignmentPick = undefined;
            } else {
                this.pendingAlignmentPick = { Side: side, AlignmentRowNumber: rowNumber };
            }
            return;
        }
        var point: AlignmentPoint = {
            Left: rowNumber, Right: this.pendingAlignmentPick.AlignmentRowNumber };
        if (side === "right") {
            point = { Left: this.pendingAlignmentPick.AlignmentRowNumber, Right: rowNumber };
        }
        this.pendingAlignmentPick = undefined;
        this.alignPoints = [...this.alignPoints, point];
    }

    // dropLineAlignmentAt removes the alignment the line is part of, and says
    // whether there was one.
    private dropLineAlignmentAt(side: Side, rowNumber: number): boolean {
        const kept = this.alignPoints.filter(function (p) {
            if (side === "left") {
                return p.Left !== rowNumber;
            }
            return p.Right !== rowNumber;
        });
        if (kept.length === this.alignPoints.length) {
            return false;
        }
        this.alignPoints = kept;
        return true;
    }

    private aligmentPickerIsPicked(side: Side, rowNumber: number): boolean {
        if (this.pendingAlignmentPick === undefined) {
            return false;
        }
        return this.pendingAlignmentPick.Side === side
            && this.pendingAlignmentPick.AlignmentRowNumber === rowNumber;
    }

    private rowIsAligned(side: Side, rowNumber: number): boolean {
        return this.alignPoints.some(function (p) {
            if (side === "left") {
                return p.Left === rowNumber;
            }
            return p.Right === rowNumber;
        });
    }
    private getDiffLine(row: AlignedRow, allRows: LeftAndRightLines,
        leftLang: string, rightLang: string): DiffLine {
        // If both sides have content
        if (row.Left !== undefined && row.Right !== undefined) {
            const rawLeft = allRows.Left.Raw[row.Left];
            const rawRight = allRows.Right.Raw[row.Right];
            var leftHtml = this.highlightAndSanitize(rawLeft, leftLang);
            var rightHtml = "";
            if (rawLeft === rawRight && leftLang === rightLang) {
                rightHtml = leftHtml; // no need to call highlightAndSanitize
            } else {
                rightHtml = this.highlightAndSanitize(rawRight, rightLang);
            }
            var diffLineType: DiffLine["type"] = "unchanged";
            if (rawLeft !== rawRight) {
                diffLineType = "left-right";
                const h = HighlightCharsAndSanitize(row.LeftRanges,
                    row.RightRanges,
                    leftHtml, rightHtml, charChangedClass);
                leftHtml = h.LeftHighlightedSanitized;
                rightHtml = h.RightHighlightedSanitized;
            }
            return {
                leftAlignmentRowNumber: row.Left,
                rightAlignmentRowNumber: row.Right,
                leftNum: allRows.Left.Nums[row.Left],
                rawLeftText: rawLeft,
                highlightedSanitizedLeftHtml: leftHtml,
                rightNum: allRows.Right.Nums[row.Right],
                rawRightText: rawRight,
                highlightedSanitizedRightHtml: rightHtml,
                type: diffLineType,
            };
        }
        // If only the left has content
        if (row.Left !== undefined) {
            const raw = allRows.Left.Raw[row.Left];
            var html = this.highlightAndSanitize(raw, leftLang)
            html = HighlightLineCharsAndSanitize(
                html, row.LeftRanges, charChangedClass)
            return {
                leftAlignmentRowNumber: row.Left,
                rightAlignmentRowNumber: undefined,
                leftNum: allRows.Left.Nums[row.Left],
                rawLeftText: raw,
                highlightedSanitizedLeftHtml: html,
                rightNum: "",
                rawRightText: "",
                highlightedSanitizedRightHtml: "",
                type: "left-only",
            };
        }
        if (row.Right === undefined){
            // This should never happen.
            // A row always has a line on at least one of the sides.
            throw "row without Left nor Right number"
        }
        const rightNumber = row.Right;
        const raw = allRows.Right.Raw[rightNumber];
        var html = this.highlightAndSanitize(raw, rightLang)
        html = HighlightLineCharsAndSanitize(
            html, row.RightRanges, charChangedClass)
        return {
            leftAlignmentRowNumber: undefined,
            rightAlignmentRowNumber: rightNumber,
            leftNum: "",
            rawLeftText: "",
            highlightedSanitizedLeftHtml: "",
            rightNum: allRows.Right.Nums[rightNumber],
            rawRightText: raw,
            highlightedSanitizedRightHtml: html,
            type: "right-only",
        };
    }

    private expandBlock(idx: number) {
        const nExpand = 20;
        const block = this.blocks[idx];
        if (!block.IsHidden()) {
            throw "tried to expand non context block"
        };

        const newTopBlock = new Block(this.nContextLines)
        const newBottomBlock = new Block(this.nContextLines)

        // This is an edge case that happens if there's only a central block;
        // which happens when the files are identical. 
        const onlyOneBlock = this.blocks.length == 1


        // Don't add a new top block on the block that is at the top
        // of the diff (as there's no code above it).
        if (idx != 0 || onlyOneBlock){
            const firstLines = block.PopFirst(nExpand)
            newTopBlock.ForceShow()
            for (const line of firstLines) {
                newTopBlock.Push(line)
            }
        }
        // Don't add a bottom block after the bottom-most block (as there's
        // no code under it).
        if (idx != this.blocks.length -1 || onlyOneBlock){
            const lastLines = block.PopLast(nExpand);
            newBottomBlock.ForceShow()
            for (const line of lastLines) {
                newBottomBlock.Push(line)
            }
        }

        const newBlocks = [...this.blocks.slice(0, idx)]
        if (newTopBlock.Size() > 0){
            newBlocks.push(newTopBlock)
        }
        if (block.Size() > 0){
            newBlocks.push(block)
        }
        if (newBottomBlock.Size() > 0) {
            newBlocks.push(newBottomBlock)
        }
        newBlocks.push(...this.blocks.slice(idx + 1))
        this.blocks = newBlocks;
    }
    private renderBlock(b: Block, idx: number){
        if (b.IsHidden()){
            return html`
            <tr>
            <td colspan=4 class="show-more" @click=${()=>this.expandBlock(idx)}>
                <div class="show-more-btn-wrapper">
                    <button class="show-more-btn">⋯ View more ⋯</button>
                </div>
            </td>
            </tr>
            `
        }
        return b.GetAll().map( (r) => html`
        <tr>
        <td class="line-num">
            <div class="line-num-inner">
                ${this.renderGutter("left", r, r.leftAlignmentRowNumber)}
            </div>
        </td>
        <td class="line-content left ${this.leftColorClass(r)} hljs"><pre><code>${unsafeHTML(r.highlightedSanitizedLeftHtml)}</code></pre></td>
        <td class="line-num">
            <div class="line-num-inner">
                ${this.renderGutter("right", r, r.rightAlignmentRowNumber)}
            </div>
        </td>
        <td class="line-content right ${this.rightColorClass(r)} hljs"><pre><code>${unsafeHTML(r.highlightedSanitizedRightHtml)}</code></pre></td>
        </tr>
        ${this.renderLineSlotTableRowIfExists(r)}
         `
        )
    }

    // draws the table row (tr) holding the slots of the lines of the row above it
    // if LeftSlotLines/RightSlotLines says there must be a slot on that line
    private renderLineSlotTableRowIfExists(r: DiffLine) {
        const slots = this.getLineSlotNames(r.leftNum, r.rightNum)
        if (slots.LeftLineSlotName === "" && slots.RightLineSlotName === ""){
            return html``
        }
        var leftSlot = html``
        if (slots.LeftLineSlotName !== "") {
            leftSlot = html`<slot name=${slots.LeftLineSlotName}></slot>`
        }
        var rightSlot = html``
        if (slots.RightLineSlotName !== "") {
            rightSlot = html`<slot name=${slots.RightLineSlotName}></slot>`
        }
        return html`
        <tr class="slot-row">
            <td class="slot-cell" colspan="2">${leftSlot}</td>
            <td class="slot-cell" colspan="2">${rightSlot}</td>
        </tr>
        `
    }
    private getLineSlotNames(
        leftNum: number | "", rightNum: number | ""): LineSlotNames {
        var leftLineSlotName = ""
        if (leftNum !== "" && this.LeftSlotLines.includes(leftNum)) {
            leftLineSlotName = DiffDisplay.LeftLineSlotName(leftNum)
        }
        var rightLineSlotName = ""
        if (rightNum !== "" && this.RightSlotLines.includes(rightNum)) {
            rightLineSlotName = DiffDisplay.RightLineSlotName(rightNum)
        }
        return { LeftLineSlotName: leftLineSlotName, RightLineSlotName: rightLineSlotName }
    }

    private renderRequestLineSlotBtn(side: Side, leftNum: number | "",
        rightNum: number | "") {
        if (!requestLineSlotBtnIsEnabled || side === "left") {
            return html``
        }
        return html`
        <button class="request-line-slot-btn" title="Add comment"
            @click=${() => this.requestLineSlot(leftNum, rightNum)}>+</button>
        `
    }

    private requestLineSlot(leftLine: number|"", rightLine: number|"") {
        if (leftLine == "" && rightLine == ""){return}
        this.dispatchEvent(new CustomEvent<RequestLineSlot>("request-line-slot", {
            detail: { LeftLine: leftLine, RightLine: rightLine },
            bubbles: true,
            composed: true,
        }))
    }

    // renderGutter draws the line number, and the button that picks the line
    // to be put on the same row as one on the other side.
    private renderGutter(side: Side, r: DiffLine, alignmentRowNumber: number | undefined) {
        let num: number | "" = ""
        if (side == "left"){
            num = r.leftNum
        }
        if (side == "right"){
            num = r.rightNum
        }
        if (alignmentRowNumber === undefined && num !== "" || alignmentRowNumber !== undefined && num === ""){
            // should NEVER happen
            throw "mismatch alignmentRowNumber and num nullness"
        }
        // Some rows are empty in one of the sides
        if (alignmentRowNumber === undefined){
            return html``
        }
        var cls = "pick-alignment-btn"
        var title = "Align this line with one on the other side"
        if (this.rowIsAligned(side, alignmentRowNumber)) {
            cls = "pick-alignment-btn aligned"
            title = "Aligned. Click to undo"
        } else if (this.aligmentPickerIsPicked(side, alignmentRowNumber)) {
            cls = "pick-alignment-btn picked"
            title = "Picked. Now pick the line it should face, or click again to drop it"
        } else if (this.pendingAlignmentPick !== undefined
            && this.pendingAlignmentPick.Side !== side) {
            cls = "pick-alignment-btn awaiting"
            title = "Align the picked line with this one"
        }
        return html`
        ${this.renderRequestLineSlotBtn(side, r.leftNum, r.rightNum)}
        <button class="${cls}" title="${title}"
            @click=${()=>this.pickLineAlignment(side, alignmentRowNumber)}>⇔</button>
        <span>${num}</span>
        `
    }

    private getBlocks(lines: DiffLine[]): Block[] {
        // Number of lines added before and after 
        const b: Block[] = []
        let currentBlock: Block = new Block(this.nContextLines)
        for (const line of lines) {
            if (currentBlock.CanPush(line)) {
                currentBlock.Push(line)
                continue
            }

            // If we can't push an unchanged line, it means the current block
            // has all the changes and the trailing lines. So we just need
            // to add it to the array we'll return and keep going
            if (line.type === "unchanged"){
                b.push(currentBlock)
                currentBlock = new Block(this.nContextLines)
                currentBlock.Push(line)
                continue
            }

            // The incoming line is a changed one. We must pop some lines
            // from the current block so they serve as context
            let ctxLines = currentBlock.PopLast(this.nContextLines)
            // Now that we have those lines in a variable, add the current block
            // to the results.
            // Then reset the current block, add the ctx lines we just popped,
            // and finally the incoming line
            if (currentBlock.Size() > 0){
                b.push(currentBlock)
            }
            currentBlock = new Block(this.nContextLines)
            for (const ctxLine of ctxLines){
                currentBlock.Push(ctxLine)
            }
            currentBlock.Push(line)
        }
        if (currentBlock.Size() > 0) {
            b.push(currentBlock)
        }
        return b
    }

    private onTableMouseDown(e: MouseEvent) {
        const table = e.currentTarget as HTMLTableElement;
        table.classList.remove("selecting-left", "selecting-right");
        const target = e.target as HTMLElement;
        // Get the cell because it will contain left/right class.
        // We'll use it to discover which side was clickd.
        const cell = target.closest("td.line-content");
        if (cell === null) {
            return;
        }
        if (cell.classList.contains("left")) {
            table.classList.add("selecting-left");
        } else {
            table.classList.add("selecting-right");
        }
    }

    private leftColorClass(l: DiffLine): string{
        if (l.type == "left-only" || l.type == "left-right"){
            return "deleted-line"
        }
        return ""
    }
    private rightColorClass(l: DiffLine): string {
        if (l.type == "right-only" || l.type == "left-right") {
            return "added-line"
        }
        return ""
    }

    private highlightAndSanitize(code: string, lang: string): string {
        if (lang == ""){
            return DOMPurify.sanitize(code);
        }
        const { value } = hljs.highlight(code, { language: lang });
        return DOMPurify.sanitize(value);
    }

    private static getFileLanguage(filename: string): (
        "go" | "javascript" | "typescript" |
        "markdown" | "json" | "xml" | "css" | 
        "scss" | "yaml" | "less" |""){
        if (filename.endsWith(".go")){
            return "go"
        }
        if (filename.endsWith(".js")) {
            return "javascript"
        }
        if (filename.endsWith(".ts") || filename.endsWith(".tsx")) {
            return "typescript"
        }
        if (filename.endsWith(".md")) {
            return "markdown"
        }
        if (filename.endsWith(".json")) {
            return "json"
        }
        if (filename.endsWith(".xml") || filename.endsWith(".html")) {
            return "xml"
        }
        if (filename.endsWith(".css")){
            return "css";
        } 
        if (filename.endsWith(".scss") || filename.endsWith(".sass")){
            return "scss";
        }
        if (filename.endsWith(".yml") || filename.endsWith(".yaml")) {
            return "yaml";
        }
        if (filename.endsWith(".less")){
            return "less"
        }
        return ""
    }

    OnThemeChanged(oldTheme: Theme, newTheme: Theme){
        this.theme = newTheme
    }

}

customElements.define('diff-display', DiffDisplay);
declare global {
    interface HTMLElementTagNameMap {
        'diff-display': DiffDisplay;
    }
}

// A set of DiffLines that are displayed/hidden together
export class Block {
    constructor(nContextLines: number){
        this.nContextLinesToUse = nContextLines
        this.lines_ = []
        this.linesStartIndex = 0
        this.changedLinesCount = 0
        this.nLeadingContextLines = 0
        this.nTrailingContextLines = 0
        this.forceShow = false
    }
    public CanPush(d: DiffLine): boolean {
        const hasChangedLine = this.changedLinesCount != 0
        // Context-only blocks can always grow
        if (!hasChangedLine && d.type === "unchanged"){
            return true
        }
        // If it already has a changed line, we can keep adding until
        // we have enough trailing context lines
        if (hasChangedLine){
            return this.nTrailingContextLines < this.nContextLinesToUse
        }
        // If there isn't a changed line yet, we can keep adding until there
        // we have enough leading context lines
        return this.nLeadingContextLines < this.nContextLinesToUse
    }
    public Push(l: DiffLine){
        const hasChangedLine = this.changedLinesCount != 0
        if (hasChangedLine && l.type === "unchanged"){
            this.nTrailingContextLines += 1
        }
        if (!hasChangedLine && l.type === "unchanged"){
            this.nLeadingContextLines += 1
        }
        if (l.type != "unchanged"){
            this.changedLinesCount += 1
        }
        this.lines_.push(l)
    }
    public Size(): number{
        return (this.lines_.length - this.linesStartIndex)
    }
    // Returns if this block is hidden or not.
    // Blocks which contain changed lines are never hidden.
    // Blocks that only contain unchanged lines are hidden by default, but
    // Are shown if `ForceShow` is called.
    public IsHidden(): boolean{
        if (this.forceShow){
            return false
        }
        return this.changedLinesCount == 0
    }
    // See IsHidden()
    public ForceShow() {
        this.forceShow = true
    }
    public GetAll(): DiffLine[]{
        // TODO: This is O(n). We can probably delete this and make something
        // better.
        return this.lines_.slice(this.linesStartIndex)
    }
    // Pop up to n (ok if n is larger).
    // They are returned in the order they appear.
    // E.g.: [1, 2, 3, 4].PopLast(2) = [3, 4]
    public PopLast(n: number): DiffLine[] {
        let popped: DiffLine[] = []
        for (let i = 0; i< n; i++){
            if (this.Size() == 0){
                break
            }
            if (this.lines_[this.lines_.length -1].type != "unchanged") {
                this.changedLinesCount -= 1
            }
            if (this.lines_[this.lines_.length -1].type == "unchanged"){
                if (this.nTrailingContextLines > 0){
                    this.nTrailingContextLines -= 1
                }else{
                    this.nLeadingContextLines -= 1
                }
            }
            popped.push(this.lines_.pop()!)
        }
        return popped.reverse()
    }
    // Similar to PopLast but for the first n
    public PopFirst(n: number): DiffLine[] {
        let popped: DiffLine[] = []
        for (let i_ = 0; i_ < n; i_++) {
            if (this.Size() == 0) {
                break
            }
            if (this.lines_[this.linesStartIndex].type != "unchanged") {
                this.changedLinesCount -= 1
            }
            if (this.lines_[this.linesStartIndex].type == "unchanged") {
                if (this.nLeadingContextLines > 0) {
                    this.nLeadingContextLines -= 1
                } else {
                    this.nTrailingContextLines -= 1
                }
            }
            popped.push(this.lines_[this.linesStartIndex])
            this.linesStartIndex += 1
        }
        // If we're left with only "trailing lines", lets just call them
        // leading
        if (this.nLeadingContextLines == 0 && this.nTrailingContextLines > 0){
            this.nLeadingContextLines = this.nTrailingContextLines
            this.nTrailingContextLines = 0
        }

        return popped
    }

    // Number of context lines that should appear before and after the changes
    private readonly nContextLinesToUse: number
    // Number of lines that were added after the changes were already added
    private nTrailingContextLines: number
    // Number of lines added before any changed line
    private nLeadingContextLines: number
    // We use a linesStartIndex just to get an O(1) for popping the first entry
    private lines_: DiffLine[]
    private linesStartIndex: number

    private changedLinesCount: number
    private forceShow: boolean
}

type Side = "left" | "right"

// LineSlotNames contains the name of the line slots (left-line-N/right-line-N)
// of each side. Empty string is used if there is no slot on one side.
interface LineSlotNames {
    // Empty string if there is no slot on left side
    LeftLineSlotName: string
    // Empty string if there is no slot on right side
    RightLineSlotName: string
}

interface PickedAlignmentLine {
    Side: Side;
    AlignmentRowNumber: number;
}

interface DiffLine {
    leftAlignmentRowNumber: number | undefined;
    rightAlignmentRowNumber: number | undefined;
    leftNum: number | "";
    rawLeftText: string;
    highlightedSanitizedLeftHtml: string;
    rightNum: number | "";
    rawRightText: string;
    highlightedSanitizedRightHtml: string;
    type: "unchanged" | "left-only" | "right-only" | "left-right"
}

function getNumOfDigitsOfMaxRow(rows: DiffLine[]): number{
    // We can leverage the fact that rows start at line 0 and advance towards
    // the end, so we start from the end and get the max of left and right
    let maxRight = 0;
    let maxLeft = 0;
    for (let i = rows.length-1; i >= 0; i--) {
        const r = rows[i]
        if (typeof r.leftNum === "number" && r.leftNum > maxLeft) maxLeft = r.leftNum;
        if (typeof r.rightNum === "number" && r.rightNum > maxRight) maxRight = r.rightNum;

        if (maxRight>0 && maxLeft>0){
            break
        }
    }
    return String(Math.max(maxLeft, maxRight)).length;
}