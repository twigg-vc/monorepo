import { expect } from '@open-wc/testing';
import { DefaultLinesDiffComputer } from './vscode-diff';

// Sanity checks that the vendored VS Code diff engine behaves as we expect.
describe('vendored vscode-diff DefaultLinesDiffComputer', () => {
    const options = {
        ignoreTrimWhitespace: false,
        computeMoves: false,
        maxComputationTimeMs: 0,
    };

    it('reports no changes for identical inputs', () => {
        const computer = new DefaultLinesDiffComputer();
        const lines = ['a', 'b', 'c'];
        const result = computer.computeDiff(lines, lines, options);
        expect(result.changes).to.have.length(0);
    });

    it('maps a modified line with 1-based line ranges', () => {
        const computer = new DefaultLinesDiffComputer();
        const result = computer.computeDiff(
            ['a', 'b', 'c'],
            ['a', 'x', 'c'],
            options);
        expect(result.changes).to.have.length(1);
        const change = result.changes[0];
        expect(change.original.startLineNumber).to.equal(2);
        expect(change.original.endLineNumberExclusive).to.equal(3);
        expect(change.modified.startLineNumber).to.equal(2);
        expect(change.modified.endLineNumberExclusive).to.equal(3);
    });

    it('maps a pure insertion to an empty original range', () => {
        const computer = new DefaultLinesDiffComputer();
        const result = computer.computeDiff(
            ['a', 'b'],
            ['a', 'x', 'b'],
            options);
        expect(result.changes).to.have.length(1);
        const change = result.changes[0];
        expect(change.original.startLineNumber).to.equal(2);
        expect(change.original.endLineNumberExclusive).to.equal(2);
        expect(change.modified.startLineNumber).to.equal(2);
        expect(change.modified.endLineNumberExclusive).to.equal(3);
    });

    it('computes character-level inner changes with 1-based columns', () => {
        const computer = new DefaultLinesDiffComputer();
        const result = computer.computeDiff(
            ['function foo() {'],
            ['function bar() {'],
            options);
        expect(result.changes).to.have.length(1);
        const inner = result.changes[0].innerChanges!;
        expect(inner).to.have.length(1);
        // "foo" -> "bar": columns 10-13 on both sides.
        expect(inner[0].originalRange.startColumn).to.equal(10);
        expect(inner[0].originalRange.endColumn).to.equal(13);
        expect(inner[0].modifiedRange.startColumn).to.equal(10);
        expect(inner[0].modifiedRange.endColumn).to.equal(13);
    });

    it('aligns unchanged lines inside a changed region instead of pairing positionally', () => {
        // A naive positional pairing would pair "a" with "x" and "b" with
        // "a". The engine keeps "a" and "b" aligned and reports the two
        // insertions around them: both changes have an empty original range.
        const computer = new DefaultLinesDiffComputer();
        const result = computer.computeDiff(
            ['a', 'b'],
            ['x', 'a', 'b', 'y'],
            options);
        expect(result.changes).to.have.length(2);
        for (const change of result.changes) {
            expect(change.original.startLineNumber).to.equal(change.original.endLineNumberExclusive);
        }
    });
});
