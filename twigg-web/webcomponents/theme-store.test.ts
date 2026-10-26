import { expect } from '@open-wc/testing';
import { ThemeStore } from './theme-store';

describe('ThemeStore', () => {
    afterEach(() => {
        localStorage.removeItem("theme")
        document.documentElement.className = ''
    })

    it('uses the theme URL param without overwriting the saved theme', () => {
        localStorage.setItem("theme", "light")
        const s = new ThemeStore()
        s.Init("?theme=dark")
        expect(s.GetTheme()).to.equal("dark")
        expect(document.documentElement.className).to.equal("dark")
        expect(localStorage.getItem("theme")).to.equal("light")
    })

    it('ignores an unknown theme URL param', () => {
        localStorage.setItem("theme", "light")
        const s = new ThemeStore()
        s.Init("?theme=pink")
        expect(s.GetTheme()).to.equal("light")
    })
})
