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
    Init(){
        if (this.IsInit){
            return
        }
        const savedTheme = localStorage.getItem(themeLocalStorageId);
        if (savedTheme) {
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
        if (t == this.Theme){
            return
        }
        setThemeOnDocumentAndLocalStorage(t)
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

function setThemeOnDocumentAndLocalStorage(theme: Theme): Theme{
    document.documentElement.className = '';
    document.documentElement.classList.toggle(theme);
    localStorage.setItem(themeLocalStorageId, theme);
    return theme
}
// function restoreThemeOrSetDefault(): Theme{
//     const savedTheme = localStorage.getItem(themeLocalStorageId);
//     if (savedTheme) {
//         return setThemeOnDocumentAndLocalStorage(savedTheme as Theme)
//     } else {
//         return setThemeOnDocumentAndLocalStorage("dark")
//     }
// }

const themeLocalStorageId = "theme"