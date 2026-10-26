export type Theme = "dark" | "light"

export interface ThemeObserver{
    OnThemeChanged(oldTheme: Theme, newTheme: Theme)
}

export class ThemeStore {
    IsInit: boolean
    Theme: Theme
    Observers: ThemeObserver[]
    constructor(){
        this.IsInit = false
        this.Theme = undefined
        this.Observers = []
    }
    // Ensures the instance is properly initialized. Ok to call many times.
    // A "theme" URL param (e.g. ?theme=dark) wins over the saved theme
    // without overwriting it.
    Init(urlSearch: string = window.location.search){
        if (this.IsInit){
            return
        }
        const urlTheme = new URLSearchParams(urlSearch).get(themeUrlParam)
        const savedTheme = localStorage.getItem(themeLocalStorageId);
        if (urlTheme == "dark" || urlTheme == "light") {
            this.setTheme(urlTheme, false)
        } else if (savedTheme) {
            this.SetTheme(savedTheme as Theme)
        } else {
            this.SetTheme("dark")
        }
        this.IsInit = true
    }
    GetTheme(): Theme{
        return this.Theme
    }
    SetTheme(t: Theme){
        this.setTheme(t, true)
    }
    private setTheme(t: Theme, save: boolean){
        if (t == this.Theme){
            return
        }
        document.documentElement.className = '';
        document.documentElement.classList.toggle(t);
        if (save) {
            localStorage.setItem(themeLocalStorageId, t);
        }
        const oldTheme = this.Theme
        this.Theme = t
        for (const obs of this.Observers){
            obs.OnThemeChanged(oldTheme, this.Theme)
        }
    }
    AddObserver(obs: ThemeObserver){
        this.Observers.push(obs)
    }
}

export const ThemeStoreSingleton = new ThemeStore()

// function restoreThemeOrSetDefault(): Theme{
//     const savedTheme = localStorage.getItem(themeLocalStorageId);
//     if (savedTheme) {
//         return setThemeOnDocumentAndLocalStorage(savedTheme as Theme)
//     } else {
//         return setThemeOnDocumentAndLocalStorage("dark")
//     }
// }

const themeLocalStorageId = "theme"
const themeUrlParam = "theme"