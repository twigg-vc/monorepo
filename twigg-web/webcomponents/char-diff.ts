// Intra-line character diff helpers used by diff-display to emphasize
// which characters of a modified line actually changed.

import { CharRange } from './differs';

import DOMPurify from 'dompurify';

export interface CharHighlight{
    LeftHighlightedSanitized: string;
    RightHighlightedSanitized: string;
}

export function HighlightCharsAndSanitize(leftRanges: CharRange[], rightRanges: CharRange[], leftHtml: string, rightHtml: string, highlightClass: string): CharHighlight{
    const left = HighlightLineCharsAndSanitize(leftHtml, leftRanges, highlightClass)
    const right = HighlightLineCharsAndSanitize(rightHtml, rightRanges, highlightClass)
    return { LeftHighlightedSanitized: left, RightHighlightedSanitized: right }
}

export function HighlightLineCharsAndSanitize(html: string, ranges: CharRange[], highlightClass: string): string {
    // DOMPurify again just in case WrapCharChanges has some bug that breaks the sanitization
    return DOMPurify.sanitize(WrapCharChanges(html, ranges, highlightClass))
}


// Wraps each character range of `htmlStr`'s text content in a
// `<span class=className>`, preserving the existing markup.
// The range offsets are relative to the text content (tags and attributes
// don't count), so ranges computed on a raw line of code can be applied to
// the syntax-highlighted HTML of that same line. If any range extends past
// the text content, the two are out of sync and the highlight would land on
// the wrong characters, so the input is returned unchanged.
// The ranges must not overlap each other; empty ranges are skipped.
// `htmlStr` must already be sanitized. The output is semantically equivalent
// HTML, but not necessarily byte-identical: parsing and re-serializing may
// normalize entities and attribute quoting.
function WrapCharChanges(htmlStr: string, ranges: CharRange[], className: string): string {
    const nonEmpty = ranges.filter(r => r.Start < r.End);
    if (nonEmpty.length === 0) {
        return htmlStr;
    }
    const tpl = document.createElement("template");
    tpl.innerHTML = htmlStr;

    const walker = document.createTreeWalker(tpl.content, NodeFilter.SHOW_TEXT);
    let totalTextLength = 0;
    let n = walker.nextNode();
    while (n !== null) {
        totalTextLength += (n as Text).data.length;
        n = walker.nextNode();
    }
    for (const r of nonEmpty) {
        if (r.End > totalTextLength) {
            return htmlStr;
        }
    }

    for (const r of nonEmpty) {
        wrapOneRange(tpl.content, r, className);
    }
    return tpl.innerHTML;
}

// Wraps the character range `r` of `root`'s text content in a
// `<span class=className>`. Wrapping earlier ranges splits text nodes and
// inserts spans, but never changes the text content, so the offsets stay
// valid across calls; the text nodes just need to be re-collected each time.
function wrapOneRange(root: DocumentFragment, r: CharRange, className: string) {
    // Collect the text nodes first: wrapping mutates the tree and would
    // confuse a live walk.
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    const textNodes: Text[] = [];
    let n = walker.nextNode();
    while (n !== null) {
        textNodes.push(n as Text);
        n = walker.nextNode();
    }

    let offset = 0;
    for (const textNode of textNodes) {
        const nodeStart = offset;
        if (nodeStart >= r.End) {
            break;
        }
        const nodeEnd = offset + textNode.data.length;
        offset = nodeEnd;

        const overlapStart = Math.max(r.Start, nodeStart);
        const overlapEnd = Math.min(r.End, nodeEnd);
        if (overlapStart >= overlapEnd) {
            continue;
        }

        // Split the node so `middle` holds exactly the overlapping text,
        // then wrap it.
        let middle = textNode;
        if (overlapStart > nodeStart) {
            middle = middle.splitText(overlapStart - nodeStart);
        }
        if (overlapEnd < nodeEnd) {
            middle.splitText(overlapEnd - overlapStart);
        }
        const span = document.createElement("span");
        span.className = className;
        middle.parentNode!.replaceChild(span, middle);
        span.appendChild(middle);
    }
}