import { expect } from '@open-wc/testing';
import { LinkifyCommitsAndBugs } from './md-anchors';

describe('LinkifyCommitsAndBugs', () => {
    it('links a basic commit ref', () => {
        const h = LinkifyCommitsAndBugs('see c/7 for details', 'acme', 'widgets');
        expect(h).to.equal('see [c/7](/acme/widgets/c/7) for details');
    });

    it('links a basic bug ref', () => {
        const h = LinkifyCommitsAndBugs('fixes b/17', 'acme', 'widgets');
        expect(h).to.equal('fixes [b/17](/acme/widgets/b/17)');
    });

    it('links a commit ref with a version suffix, dropping the version from the href', () => {
        const h = LinkifyCommitsAndBugs('rebased onto c/7v2', 'acme', 'widgets');
        expect(h).to.equal('rebased onto [c/7v2](/acme/widgets/c/7)');
    });

    it('is case-insensitive', () => {
        const h = LinkifyCommitsAndBugs('see C/7 and B/17', 'acme', 'widgets');
        expect(h).to.equal('see [C/7](/acme/widgets/c/7) and [B/17](/acme/widgets/b/17)');
    });

    it('leaves a fenced code block untouched', () => {
        const input = 'text c/7\n```\nc/7 and b/17\n```\nmore c/8';
        const h = LinkifyCommitsAndBugs(input, 'acme', 'widgets');
        expect(h).to.equal(
            'text [c/7](/acme/widgets/c/7)\n```\nc/7 and b/17\n```\nmore [c/8](/acme/widgets/c/8)');
    });

    it('leaves an inline code span untouched', () => {
        const h = LinkifyCommitsAndBugs('run `tw goto c/7` then c/8', 'acme', 'widgets');
        expect(h).to.equal('run `tw goto c/7` then [c/8](/acme/widgets/c/8)');
    });

    it('leaves an existing markdown link untouched', () => {
        const h = LinkifyCommitsAndBugs('[c/7](/acme/widgets/c/7) and c/8', 'acme', 'widgets');
        expect(h).to.equal('[c/7](/acme/widgets/c/7) and [c/8](/acme/widgets/c/8)');
    });

    it('does not match a ref embedded in a larger word', () => {
        const h = LinkifyCommitsAndBugs('abc/7 stays as is', 'acme', 'widgets');
        expect(h).to.equal('abc/7 stays as is');
    });

    it('does not re-link a ref that is already part of a url path', () => {
        const h = LinkifyCommitsAndBugs('see /acme/widgets/c/7', 'acme', 'widgets');
        expect(h).to.equal('see /acme/widgets/c/7');
    });

    it('links multiple refs in the same string', () => {
        const h = LinkifyCommitsAndBugs('c/1 fixes b/2, see also c/3', 'acme', 'widgets');
        expect(h).to.equal(
            '[c/1](/acme/widgets/c/1) fixes [b/2](/acme/widgets/b/2), see also [c/3](/acme/widgets/c/3)');
    });

    it('leaves text with no refs untouched', () => {
        const h = LinkifyCommitsAndBugs('nothing to link here', 'acme', 'widgets');
        expect(h).to.equal('nothing to link here');
    });
});
