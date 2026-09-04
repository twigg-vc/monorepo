import { expect } from '@open-wc/testing';
import { HighlightCharsAndSanitize, HighlightLineCharsAndSanitize } from './char-diff';

describe('HighlightLineCharsAndSanitize', () => {
    it('wraps the given range of a plain line', () => {
        const h = HighlightLineCharsAndSanitize("abcdef", [{ Start: 2, End: 3 }], "char-changed");
        expect(h).to.equal('ab<span class="char-changed">c</span>def');
    });

    it('wraps the whole line', () => {
        const h = HighlightLineCharsAndSanitize("abc", [{ Start: 0, End: 3 }], "char-changed");
        expect(h).to.equal('<span class="char-changed">abc</span>');
    });

    it('leaves a line with no ranges untouched', () => {
        const h = HighlightLineCharsAndSanitize("abc", [], "char-changed");
        expect(h).to.equal("abc");
    });

    it('leaves an empty range untouched', () => {
        const h = HighlightLineCharsAndSanitize("abc", [{ Start: 1, End: 1 }], "char-changed");
        expect(h).to.equal("abc");
    });

    it('wraps several ranges on the same line', () => {
        const h = HighlightLineCharsAndSanitize("The quick brown fox",
            [{ Start: 4, End: 9 }, { Start: 16, End: 19 }], "char-changed");
        expect(h).to.equal(
            'The <span class="char-changed">quick</span> brown <span class="char-changed">fox</span>');
    });

    it('wraps several ranges within the same markup element', () => {
        const h = HighlightLineCharsAndSanitize('<span class="k">abcdefg</span>',
            [{ Start: 1, End: 2 }, { Start: 5, End: 6 }], "char-changed");
        expect(h).to.equal(
            '<span class="k">a<span class="char-changed">b</span>cde<span class="char-changed">f</span>g</span>');
    });

    it('keeps the syntax highlighting markup around the wrapped chars', () => {
        const h = HighlightLineCharsAndSanitize('<span class="hljs-keyword">return</span>',
            [{ Start: 3, End: 4 }], "char-changed");
        expect(h).to.equal('<span class="hljs-keyword">ret<span class="char-changed">u</span>rn</span>');
    });

    it('wraps a range spanning multiple syntax highlighting tags', () => {
        const h = HighlightLineCharsAndSanitize('<span class="a">abc</span><span class="b">def</span>',
            [{ Start: 2, End: 4 }], "char-changed");
        expect(h).to.equal(
            '<span class="a">ab<span class="char-changed">c</span></span>' +
            '<span class="b"><span class="char-changed">d</span>ef</span>');
    });

    it('counts offsets by text content, not by markup', () => {
        // The text content is "a & b" but its HTML encodes the "&" as a
        // 5-char entity; the highlight must still land on the "&".
        const h = HighlightLineCharsAndSanitize('a &amp; b', [{ Start: 2, End: 3 }], "char-changed");
        expect(h).to.equal('a <span class="char-changed">&amp;</span> b');
    });

    it('skips the highlight when a range runs past the text content', () => {
        // The range doesn't apply to this HTML; no highlight is safer than a
        // misplaced one.
        const h = HighlightLineCharsAndSanitize("abc", [{ Start: 6, End: 8 }], "char-changed");
        expect(h).to.equal("abc");
    });

    it('uses the given highlight class', () => {
        const h = HighlightLineCharsAndSanitize("aXc", [{ Start: 1, End: 2 }], "my-class");
        expect(h).to.equal('a<span class="my-class">X</span>c');
    });

    it('sanitizes the returned HTML', () => {
        const h = HighlightLineCharsAndSanitize('<a href="x" onclick="evil()">hi</a>', [], "char-changed");
        expect(h).to.equal('<a href="x">hi</a>');
    });
});

describe('HighlightCharsAndSanitize', () => {
    it('wraps each side with its own ranges', () => {
        const h = HighlightCharsAndSanitize(
            [{ Start: 0, End: 1 }], [{ Start: 3, End: 5 }],
            "abcdef", "abcdef", "char-changed");
        expect(h.LeftHighlightedSanitized).to.equal('<span class="char-changed">a</span>bcdef');
        expect(h.RightHighlightedSanitized).to.equal('abc<span class="char-changed">de</span>f');
    });
});
