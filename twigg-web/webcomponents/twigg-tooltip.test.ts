import { expect, fixture, fixtureCleanup, html } from '@open-wc/testing';
import { TwiggTooltip } from './twigg-tooltip';
import './twigg-tooltip';

describe('twigg-tooltip', () => {
    afterEach(() => {
        fixtureCleanup()
    })

    async function tooltipAt(style: string): Promise<TwiggTooltip> {
        return fixture<TwiggTooltip>(html`
            <twigg-tooltip style=${`position: fixed; ${style}`}>
                <span>trigger</span>
                <div slot="content"><p>line 1</p><p>line 2</p><p>line 3</p></div>
            </twigg-tooltip>
        `)
    }

    function tooltipBox(el: TwiggTooltip): HTMLElement {
        return el.shadowRoot!.querySelector<HTMLElement>('.tooltip')!
    }

    async function hover(el: TwiggTooltip) {
        el.shadowRoot!.querySelector('.container')!.dispatchEvent(new Event('mouseenter'))
        await el.updateComplete
    }

    it('opens below the trigger on hover and closes on scroll', async () => {
        const el = await tooltipAt('top: 10px; left: 10px;')
        expect(getComputedStyle(tooltipBox(el)).display).to.equal('none')

        await hover(el)
        const trigger = el.shadowRoot!.querySelector('.trigger')!.getBoundingClientRect()
        expect(getComputedStyle(tooltipBox(el)).display).to.equal('block')
        expect(tooltipBox(el).getBoundingClientRect().top).to.be.at.least(trigger.bottom)

        window.dispatchEvent(new Event('scroll'))
        expect(getComputedStyle(tooltipBox(el)).display).to.equal('none')
    })

    it('opens above the trigger when there is no room below', async () => {
        const el = await tooltipAt('bottom: 10px; left: 10px;')
        await hover(el)
        const trigger = el.shadowRoot!.querySelector('.trigger')!.getBoundingClientRect()
        const box = tooltipBox(el).getBoundingClientRect()
        expect(box.bottom).to.be.at.most(trigger.top)
        expect(box.bottom).to.be.at.most(window.innerHeight)
    })

    it('paints above later siblings that create stacking contexts', async () => {
        // fixture() on a plain div waits for an animation frame, which stalls in a parallel run.
        const wrapper = document.createElement('div')
        wrapper.innerHTML = `
            <div style="position: fixed; top: 10px; left: 10px;">
                <div style="opacity: 0.5;">
                    <twigg-tooltip><span>trigger</span><div slot="content"><p>content</p></div></twigg-tooltip>
                </div>
                <div style="position: relative; z-index: 1; height: 200px; width: 400px;"></div>
            </div>
        `
        document.body.appendChild(wrapper)
        try {
            const el = wrapper.querySelector('twigg-tooltip')!
            await el.updateComplete
            await hover(el)
            const box = tooltipBox(el).getBoundingClientRect()
            const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2)
            // chai hangs serializing DOM nodes, so compare booleans.
            expect(hit !== null && el.contains(hit)).to.equal(true)
        } finally {
            wrapper.remove()
        }
    })
})
