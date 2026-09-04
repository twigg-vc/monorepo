import { html, LitElement, css } from 'lit';
import { TwiggCss } from './css';
import { ThemeSelector } from './theme-selector';
import { OrganizationNameParamName, OrganizationsPattern, UrlToGetNotifications, UrlToMarkNotificationRead, UrlToGetNotificationsUnseenCount, UrlToMarkAllNotificationsSeen, GetCsrfHeaders } from './routes';
import { DocumentationPage, HomeUrl, Logout, PostLoginUrl, TwiggLogoBlackUrl, TwiggLogoWhiteUrl, UserSettings } from './routes';
import { Theme, ThemeStoreSingleton } from './theme-store';
import { GetFeatureFlags } from './feature-flags';
import { FormatDateTime } from "./helpers";

type Notification = {
    Id: number;
    UserId: number;
    Message: string;
    AssetPath: string;
    CreatedAt: string;
    SeenAt: string;
    ReadAt: string;
};

export class NavBar extends LitElement {
	static properties = {
		Open: { type: Boolean },
        Theme: { type: String },
        UserOpen: { type: Boolean },

        NotifOpen: { type: Boolean },
        NotifIsLoading: { type: Boolean },
        Notifications: { type: Array },
        HideLoadMore: { type: Boolean },
        unseenCount: { type: Number, state: true },
	};
	declare Open: boolean;
    declare Theme: Theme;
    declare UserOpen: boolean;

    declare NotifOpen: boolean;
    declare NotifIsLoading: boolean;
    declare Notifications: Notification[];
    declare HideLoadMore: boolean;
    declare private unseenCount: number;


	constructor() {
		super();
		this.Open = false;
        this.Theme = ThemeStoreSingleton.GetTheme();
        ThemeStoreSingleton.AddObserver(this);
        this.UserOpen = false;

        this.NotifOpen = false;
        this.NotifIsLoading = false;
        this.Notifications = [];
        this.HideLoadMore = false;
        this.unseenCount = 0;
	}

	toggleMenu() {
		this.Open = !this.Open;
	}

    firstUpdated() {
        window.addEventListener('click', this._closeUserMenu);
        this.loadUnseenCount();
        this.notifyRefreshInterval = window.setInterval(() => {
            this.loadUnseenCount();
        }, 1 * 60 * 1000); // 1 minute
    }
    disconnectedCallback() {
        super.disconnectedCallback();
        window.removeEventListener('click', this._closeUserMenu);
        window.clearInterval(this.notifyRefreshInterval)
    }
    private _closeUserMenu = () => {
        this.UserOpen = false;
        this.NotifOpen = false;
    };
    private toggleUserMenu(e?: Event) {
        e?.preventDefault();
        e?.stopPropagation();
        this.UserOpen = !this.UserOpen;
        if (this.UserOpen) {
            this.NotifOpen = false
        }
    }
    private async logout(e: Event) {
        e.preventDefault();
        try{
            const res = await fetch(
                Logout, { method: 'POST', headers: GetCsrfHeaders() });
            if (!res.ok){
                throw "request failed"
            }
        }catch(e){
            alert("logout failed :(")
            return
        }
        window.location.href = PostLoginUrl;
    }
    private async toggleNotifMenu(e?: Event) {
        e?.preventDefault();
        e?.stopPropagation();
        this.NotifOpen = !this.NotifOpen;
        if (this.NotifOpen) {
            this.UserOpen = false
            await this.loadNotifications();
        }
    }
    private async loadNotifications(lastReadNotificationId?: number) {
        if (!lastReadNotificationId){
            // If we're reading the "first page" of notifications,
            // reset HideLoadMore
            this.HideLoadMore = false
        }
        this.NotifIsLoading = true;
        try {
            const res = await fetch(
                UrlToGetNotifications(lastReadNotificationId),
                { method: "GET" }
            );

            if (!res.ok) {
                console.error("failed to load notifications", res.status);
                this.NotifIsLoading = false
                return;
            }

            const data: { Notifications: Notification[], UnseenCount: number } = await res.json();

            // If we ever get no new notification, hide the load more btn
            if (data.Notifications.length == 0){
                this.HideLoadMore = true
            }

            if (lastReadNotificationId) {
                this.Notifications = [...this.Notifications, ...data.Notifications];
            } else {
                this.Notifications = data.Notifications;
            }

            // The backend marks notifications as seen on GET and returns the
            // updated count, so we update the badge here instead of a separate call.
            this.unseenCount = data.UnseenCount;

        } catch (err) {
            console.error("failed to load notifications", err);
        }
        this.NotifIsLoading = false;
    }
    private getOldestNotificationId(): number | undefined {
        if (!this.Notifications.length) {
            return undefined
        }
        return this.Notifications[this.Notifications.length - 1].Id
    }
    private async loadMoreNotifications() {
        await this.loadNotifications(this.getOldestNotificationId());
    }
    private async loadUnseenCount() {
        try {
            const res = await fetch(UrlToGetNotificationsUnseenCount(), { method: "GET" });
            if (!res.ok) {
                console.error("failed to load unseen count", res.status);
                return;
            }
            const data: { Count: number } = await res.json();
            this.unseenCount = data.Count;
        } catch (err) {
            console.error("failed to load unseen count", err);
        }
    }
    private async markAllSeen() {
        try {
            const res = await fetch(UrlToMarkAllNotificationsSeen(), {
                method: "POST",
                headers: {...GetCsrfHeaders()}
            });
            if (!res.ok) {
                console.error("mark all seen failed:", res.status);
                return;
            }
            const now = new Date().toISOString();
            this.Notifications = this.Notifications.map(n => ({
                ...n,
                SeenAt: n.SeenAt !== "" ? n.SeenAt : now,
            }));
            this.unseenCount = 0;
        } catch (err) {
            console.error("mark all seen failed:", err);
        }
    }
    private notifyRefreshInterval: number
	render() {
		return html`
		<header>
			<div class="nav" role="navigation" aria-label="Primary">
                <a href="${HomeUrl}">
                    <div class="brand">
                        <img src=${this.getLogoUrl()} alt="Twigg logo" />
                    </div>
                </a>

                <div class="nav-end">
                    <nav class="header-links">
                        ${this.renderLinks()}
                    </nav>

                    <theme-selector></theme-selector>

                    <button
                        class="btn menu-toggle"
                        @click=${this.toggleMenu}
                        aria-expanded=${this.Open ? "true" : "false"}
                        aria-controls="dropdown-links"
                    >
                    ☰
                    </button>
                </div>
			</div>
		</header>

        <nav
        id="dropdown-links"
        class="dropdown-links ${this.Open ? 'open' : ''}">
            ${this.renderLinks()}
        </nav>
	`;
	}
    private renderLinks(){
        return html`
            <a class="icon-btn" href="${HomeUrl}">
                <twigg-icon class="tab-icon" icon="Home" style="font-size: var(--space5)"></twigg-icon>
            </a>
            <div class="notif-menu">
                <div class="centralized">
                    <a
                        class="icon-btn"
                        @click=${this.toggleNotifMenu}
                        aria-haspopup="menu"
                        aria-expanded=${this.NotifOpen ? 'true' : 'false'}
                    >
                        <div class="notif-icon-wrapper">
                            <twigg-icon
                                class="tab-icon"
                                icon="Bell"
                                style="font-size: var(--space5)">
                            </twigg-icon>

                            ${this.unseenCount > 0 ? html`
                                <span class="notif-badge">
                                    ${this.unseenCount > 99 ? '99+' : this.unseenCount}
                                </span>
                            ` : null}
                        </div>

                    </a>
                </div>
                <div class="notif-menu-list nested-submenu-on-mobile ${this.NotifOpen ? 'open' : ''}" role="menu">
                    ${this.unseenCount > 0 ? html`
                        <div class="notif-actions">
                            <span class="notif-action-btn" @click=${this.markAllSeen}>Mark all as seen</span>
                        </div>
                    ` : null}
                    <div class="notify-scroll twigg-scroll">
                        ${this.renderNotifications()}
                    </div>
                </div>
            </div>
            <div class="user-menu">
                <div class="centralized">
                    <a
                        class="icon-btn"
                        @click=${this.toggleUserMenu}
                        aria-haspopup="menu"
                        aria-expanded=${this.UserOpen ? 'true' : 'false'}
                    >
                        <twigg-icon class="tab-icon" icon="User" style="font-size: var(--space5)"></twigg-icon>
                    </a>
                </div>
                <div class="user-menu-list nested-submenu-on-mobile ${this.UserOpen ? 'open' : ''}" role="menu">

                    <a role="menuitem" href="${UserSettings}">
                        <twigg-icon class="tab-icon" icon="Cog"></twigg-icon>
                        User settings
                    </a>
                    <a role="menuitem" @click=${this.logout}>
                        <twigg-icon class="tab-icon" icon="Out"></twigg-icon>
                        Log out
                    </a>
                </div>
            </div>
            ${this.renderOrgsIconBtn()}
            <a class="icon-btn" href="${DocumentationPage}">
                <twigg-icon 
                class="tab-icon" icon="Doc" title="Twigg documentation" 
                style="font-size: var(--space5)"></twigg-icon>
            </a>
        `;
    }
    private getNotifClass(n: Notification): string {
        if (n.ReadAt !== "") {
            return "read";
        }
        if (n.SeenAt === "") {
            return "not-seen";
        }
        return "seen";
    }
    private renderNotificationsLoader() {
        return html`<div class="notif-empty">Loading...</div>`
    }
    private renderNotifications() {
        if (!this.Notifications || this.Notifications.length === 0) {
            if (this.NotifIsLoading) {
                return this.renderNotificationsLoader()
            }
            return html`<div class="notif-empty">No notifications</div>`;
        }
        return html`
        ${this.Notifications.map((n) => html`
            <a
                class="notif-item ${this.getNotifClass(n)}"
                role="menuitem"
                @click=${(e: Event) => this.onNotificationClick(e, n)}
                title=${n.CreatedAt}
            >
                <div class="notif-msg">${n.Message}</div>
                <div class="notif-time">${FormatDateTime(n.CreatedAt)}</div>
            </a>
        `)}

        ${this.renderLoadMore()}
    `;
    }
    private renderLoadMore(){
        if (this.NotifIsLoading){
            return this.renderNotificationsLoader()
        }
        if (this.HideLoadMore){
            return html``
        }
        return html`
            <div
            class="notif-more"
            @click=${this.onLoadMoreClick}
            >
            Load more
            </div>
        `
    }

    private onLoadMoreClick = async (e: Event) => {
        e.preventDefault();
        e.stopPropagation();
        await this.loadMoreNotifications();
    };

    private async onNotificationClick(e: Event, n: Notification) {
        e.preventDefault();
        e.stopPropagation();

        if (!n.ReadAt) {
            try {
                const res = await fetch(UrlToMarkNotificationRead(), {
                    method: "POST",
                    headers: { ...GetCsrfHeaders(), "Content-Type": "application/json" },
                    body: JSON.stringify({ NotificationId: n.Id }),
                });
                if (!res.ok) {
                    console.error("mark read failed:", res.status)
                    return
                }
            } catch (err) {
                console.error("mark read failed:", err)
                return
            }
        }
        this.NotifOpen = false;
        window.location.href = n.AssetPath;
    }

    private formatTime(iso: string) {
        return iso;
    }

    private getLogoUrl(){
        if (this.Theme == "light"){
            return TwiggLogoBlackUrl
        }
        return TwiggLogoWhiteUrl
    }
    public OnThemeChanged(oldTheme: Theme, newTheme: Theme){
        this.Theme = newTheme
    }

    private renderOrgsIconBtn(){
        if (!GetFeatureFlags().OrganizationFeatureIsEnabled) {
            return html``
        }
        return html`
        <a class="icon-btn" href="${OrganizationsPattern}">
            <twigg-icon 
            class="tab-icon" icon="BuildingOffice2" title="organizations" 
            style="font-size: var(--space5)"></twigg-icon>
        </a>
        `
    }

	static styles = [
		TwiggCss,
		css`
		header {
			position: sticky;
			top: 0;
			border-bottom: 1px solid var(--color-border);
			z-index: 20;
			padding: var(--fixedSpace1) 0;
		}
        .centralized {
            display: flex;
            justify-content: center; /* horizontal */
            align-items: center;     /* vertical */
        }
		.nav {
			display: flex;
			align-items: center;
			justify-content: space-between;
			margin: 0 auto;
			padding: var(--fixedSpace0) var(--fixedSpace4);
		}

		.brand {
			display: flex;
            flex-direction: column;
			align-items: center;
		}
		.brand img {
			height: var(--space6);
			width: auto;
		}
        .nav-end{
            display: flex;
            align-items: center;
            gap: var(--space4);
        }
        .header-links {
            display: flex;
            align-items: center;
            gap: var(--space4);
        }
        .dropdown-links {
            display: none; /* hidden on large screens */
        }
		.menu-toggle {
			display: none;
		}

		@media (max-width: 760px) {
            .header-links {
                display: none;
            }
            .menu-toggle {
                display: inline-flex;
            }
            .dropdown-links {
                display: none;
                flex-direction: column;
                background: var(--color-surface);
                overflow: auto;
            }
            .dropdown-links.open {
                display: flex;
            }
            .dropdown-links > a {
                border-bottom: 1px solid var(--color-border);
                padding: var(--space2);
                text-align: center;
            }
            .dropdown-links div.centralized {
                border-bottom: 1px solid var(--color-border);
                padding: var(--space2);
                text-align: center;
            }
            .dropdown-links .user-menu-list {
                position: static;
                border: none;
                box-shadow: none;
                flex-direction: column;
            }
            .nested-submenu-on-mobile {
                padding-inline: var(--space5p);
            }
        }

        .user-menu {
            position: relative;     
        }

        .user-menu-list {
            position: absolute;
            right: 0;
            top: calc(100% + var(--space1));
            min-width: 160px;
            background: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius2);
            display: none;                
            z-index: 30;                  
            overflow: hidden;
        }

        .user-menu-list.open {
            display: block;               
        }

        .user-menu-list a {
            display: flex;
            padding: var(--space2) var(--space3);
            text-decoration: none;
            color: var(--color-text);
            border-bottom: 1px solid var(--color-border);
            gap: var(--space2);
        }

        .user-menu-list a:last-child {
            border-bottom: none;
        }

        .user-menu-list a:hover {
            background: var(--color-bg);
            color: var(--color-primary-pop)
        }

        .notif-menu {
            position: relative;
        }

        .notif-menu-list {
            position: absolute;
            right: 0;
            top: calc(100% + var(--space1));
            width: var(--size1);
            max-height: calc(var(--size1) + var(--space6));
            overflow: hidden;

            background: var(--color-surface);
            border: 1px solid var(--color-border);
            border-radius: var(--radius2);
            display: none;
            z-index: 40;
        }

        .notif-menu-list.open {
            display: flex;
            flex-direction: column;
        }

        .notif-empty {
            padding-left: var(--space3);
            padding-top: var(--space2);
            padding-bottom: var(--space2);
            text-align: left;
            font-size: var(--space3);
            color: var(--color-text);
            opacity: 0.8;
        }

        .notif-actions {
            padding: var(--space1) var(--space3);
            border-bottom: 1px solid var(--color-border);
        }
        .notif-action-btn {
            font-size: var(--space3);
            color: var(--color-text);
            cursor: pointer;
            opacity: 0.8;
        }
        .notif-action-btn:hover {
            transform: translateY(-1px);
            opacity: 1;
            color: var(--color-primary-pop);
        }

        .notif-item {
            display: block;
            padding: var(--space3);
            text-decoration: none;
            color: var(--color-text);
            border-bottom: 1px solid var(--color-border);
            cursor: pointer;
        }

        .notif-item:hover {
            background: var(--color-bg);
            color: var(--color-primary-pop);
            text-decoration: none;
        }

        .notif-item.not-seen .notif-msg {
            font-weight: var(--weight-bold);
            border-left: var(--fixedSpace0) solid var(--color-primary-pop);
            padding-left: var(--space2);
        }

        .notif-item.seen .notif-msg {
            font-weight: var(--weight-semi-bold);
        }

        .notif-item.read .notif-msg {
            opacity: 0.85;
        }

        .notif-msg {
            line-height: 1.25;
        }

        .notif-time {
            margin-top: var(--space1);
            font-size: var(--space3);
            opacity: 0.7;
        }
        @media (max-width: 760px) {
            .dropdown-links .notif-menu-list {
                position: static;
                width: 100%;
                max-height: none;
                border: none;
                box-shadow: none;
                border-radius: 0;
            }
        }
        .notif-icon-wrapper{
            position: relative;
        }
        .notif-badge {
            position: absolute;
            top: -4px;
            right: -3px;

            min-width: 16px;

            background: var(--color-primary-pop);
            color: white;
            font-size: 11px;
            font-weight: var(--weight-bold);
            line-height: 16px;
            text-align: center;

            border-radius: 999px;

            cursor: pointer;
        }
        .icon-btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            vertical-align: middle;
        }
        .tab-icon{
            cursor: pointer;
        }
        .notif-more {
            display: flex;
            align-items: center;
            padding: var(--space2) var(--space3);
            font-size: var(--space3);
            color: var(--color-text);
            opacity: 0.8;
            cursor: pointer;
        }
        .notif-more:hover {
            transform: translateY(-1px);
            opacity: 1;
            color: var(--color-primary-pop)
        }
        .notify-scroll {
            max-height: var(--size1);
            overflow: auto;
        }
	`];
}

customElements.define("nav-bar", NavBar);
declare global {
	interface HTMLElementTagNameMap {
		'nav-bar': NavBar;
	}
}