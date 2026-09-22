import { expect } from '@open-wc/testing';
import { ParseEnv, LooksLikeEnv } from './env-parser';

describe('ParseEnv', () => {
    it('parses simple KEY=value lines', () => {
        expect(ParseEnv("A=1\nB=two")).to.deep.equal([
            { Name: "A", Value: "1" },
            { Name: "B", Value: "two" },
        ])
    })

    it('skips blank lines, comments and lines without "="', () => {
        expect(ParseEnv("\n# comment\nJUNK\nA=1\n\n")).to.deep.equal([
            { Name: "A", Value: "1" },
        ])
    })

    it('strips "export" and surrounding whitespace', () => {
        expect(ParseEnv("export  A = 1 ")).to.deep.equal([
            { Name: "A", Value: "1" },
        ])
    })

    it('unquotes single and double quoted values, keeping inner content', () => {
        expect(ParseEnv(`A="x # y"\nB='it=s'`)).to.deep.equal([
            { Name: "A", Value: "x # y" },
            { Name: "B", Value: "it=s" },
        ])
    })

    it('keeps "=" inside unquoted values and drops trailing comments', () => {
        expect(ParseEnv("URL=postgres://u:p@h/db?x=1 # prod")).to.deep.equal([
            { Name: "URL", Value: "postgres://u:p@h/db?x=1" },
        ])
    })

    it('allows empty values', () => {
        expect(ParseEnv("A=")).to.deep.equal([{ Name: "A", Value: "" }])
    })

    it('handles windows line endings', () => {
        expect(ParseEnv("A=1\r\nB=2")).to.deep.equal([
            { Name: "A", Value: "1" },
            { Name: "B", Value: "2" },
        ])
    })
})

describe('LooksLikeEnv', () => {
    it('is false for a plain secret name', () => {
        expect(LooksLikeEnv("MY_SECRET")).to.equal(false)
    })
    it('is true when there is an "=" or several lines', () => {
        expect(LooksLikeEnv("A=1")).to.equal(true)
        expect(LooksLikeEnv("A\nB")).to.equal(true)
    })
})
