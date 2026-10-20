import { expect, fixture, fixtureCleanup, html } from '@open-wc/testing';
import { RepoDisplay } from './repo-display';
import { FeatureFlags, GetFeatureFlags, SetFeatureFlags } from './feature-flags';
import { Commit } from './interfaces';
import './repo-display';

describe('repository tab URLs', () => {
    var originalUrl: string
    var originalFlags: FeatureFlags

    beforeEach(() => {
        originalUrl = window.location.href
        originalFlags = GetFeatureFlags()
        SetFeatureFlags({ ...originalFlags, ShowBugs: true, ShowCdJobs: true, SearchCommitsUi: false })
    })

    afterEach(() => {
        fixtureCleanup()
        history.replaceState(null, '', originalUrl)
        SetFeatureFlags(originalFlags)
    })

    async function repo(search: string): Promise<RepoDisplay> {
        history.replaceState(null, '', `/owner/repo${search}`)
        return fixture<RepoDisplay>(html`
            <repo-display
                RepoOwnerName="owner"
                RepoName="repo"
                .SubmittedCommits=${[{ L: 0 } as Commit]}>
            </repo-display>
        `)
    }

    async function clickTab(element: RepoDisplay, title: string) {
        const tab = element.shadowRoot!.querySelector<HTMLElement>(`.tab[title="${title}"]`)!
        tab.click()
        await element.updateComplete
        expect(tab.classList.contains('active')).to.equal(true)
    }

    it('adds the bugs parameter and removes it when returning to commits', async () => {
        const element = await repo('?q=author%3Ame#section')
        await clickTab(element, 'Bugs')
        expect(new URLSearchParams(location.search).get('tab')).to.equal('bugs')
        await clickTab(element, 'Commits')
        expect(new URLSearchParams(location.search).has('tab')).to.equal(false)
        expect(new URLSearchParams(location.search).get('q')).to.equal('author:me')
        expect(location.hash).to.equal('#section')
        expect(location.pathname).to.equal('/owner/repo')
    })

    it('removes the parameter after opening a direct bugs link', async () => {
        const element = await repo('?tab=bugs')
        expect(element.shadowRoot!.querySelector('.tab.active')!.getAttribute('title')).to.equal('Bugs')
        await clickTab(element, 'Commits')
        expect(location.search).to.equal('')
    })

    it('writes the graph and CD parameters without adding history entries', async () => {
        const element = await repo('')
        const historyLength = history.length
        await clickTab(element, 'Commit graph')
        expect(new URLSearchParams(location.search).get('tab')).to.equal('commit-graph')
        await clickTab(element, 'CD')
        expect(new URLSearchParams(location.search).get('tab')).to.equal('cd')
        expect(history.length).to.equal(historyLength)
    })
})
