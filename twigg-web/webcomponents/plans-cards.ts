import { LitElement, html, css } from "lit";
import { CsrfFormName, GetCsrfFormValue, IsChoosingPlanForOrgParamName, OrganizationNameParamName, StripePriceIdParamName, StripeQuantityParamName, SubscribeTrialUrl, SubscribeWithStripeUrl } from "./routes";
import { User } from "./interfaces";

export class PlansCards extends LitElement {
    static properties = {
        User: { type: Object },
        SoloPrice: { type: String },
        StripeSoloPriceId: { type: String },
        TeamsPrice: { type: String },
        StripeTeamPriceId: { type: String },
        TeamQuantity: { type: Number },
        Org: { type: Object },
        IsChoosingPlanForOrg: { type: Boolean },
    };
    declare User: User;
    declare Org: User;
    declare IsChoosingPlanForOrg: boolean;
    declare SoloPrice: string;
    declare StripeSoloPriceId: string;
    declare TeamsPrice: string;
    declare StripeTeamPriceId: string;
    declare TeamQuantity: number;
    constructor() {
        super();
        this.SoloPrice = "12";
        this.StripeSoloPriceId="";
        this.TeamsPrice = "18";
        this.StripeTeamPriceId="";
        this.TeamQuantity = 0;
        this.IsChoosingPlanForOrg = false;
    }

    static styles = css`
    :host {
      display: block;
      min-height: 80vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: var(--space4);
    }
    .container {
      max-width: var(--size4);
      width: 100%;
    }
    .grid {
        display: flex;
        justify-content: center;
        flex-wrap: wrap;
        gap: var(--space4);
        max-width: 100%;
        margin: 0 auto;
    }
    .card {
        background: var(--color-surface);
        border-radius: var(--radius2);
        padding: var(--space4);
        box-shadow: var(--shadow-surface);
        display: flex;
        flex-direction: column;
        transition: transform .2s, box-shadow .2s, border-color .2s;
        box-sizing: border-box;
    }
    .card:hover {
      box-shadow: var(--shadow-pop);
      transform: translateY(-2px);
    }
    .card h2{
        margin-top: 0;
    }
    .card.teams {
        border: 2px solid var(--color-primary);
        box-sizing: border-box;
    }
    .price {
      font-size: var(--space6);
      font-weight: var(--weight-bold);
      margin-bottom: var(--space4);
      color: var(--color-text);
    }
    ul {
      margin-bottom: var(--space4);
      flex: 1;
      padding-left: var(--space3);
    }
    li {
      margin-bottom: var(--space2);
      color: var(--color-text-muted);
    }
    button {
      display: block;          /* make it block-level */
      width: 100%;  
      border-radius: var(--radius1);
      background: var(--color-primary);
      color: var(--color-text-on-primary);
      text-align: center;
      padding: var(--space2) var(--space3);
      font-weight: var(--weight-semi-bold);
      transition: background 0.2s;
      cursor: pointer;
      border: none;
      font-family: inherit;
      font-size: inherit;
      box-sizing: border-box;
    }
    button:hover {
      background: var(--color-primary-pop);
      text-decoration: none;
    }
    button.disabled,
    button:disabled {
        background: var(--color-border);
        color: var(--color-text-muted);
        cursor: not-allowed;
        box-shadow: none;
        opacity: var(--disable-opacity-value);
    }
    button.disabled:hover,
    button:disabled:hover {
        background: var(--color-border);
    }
    .quantity-label {
        display: block;
        margin-bottom: var(--space2);
        color: var(--color-text);
        font-weight: var(--weight-semi-bold);
    }
    .quantity-input {
        width: 100%;
        padding: var(--space2);
        margin-top: var(--space1);
        border-radius: var(--radius1);
        border: 1px solid var(--color-border);
        box-sizing: border-box;
        font: inherit;
    }
    /* base (mobile): 1 por linha */
    .card {
        flex: 0 1 100%;
        box-sizing: border-box;
        min-width: 0;
    }

    /* tablet: 2 por linha -> 100% menos 1 gap, dividido por 2 */
    @media (min-width: 640px) {
        .card {
            flex: 0 1 calc((100% - var(--space4)) / 2);
        }
    }

    /* desktop: 3 por linha -> 100% menos 2 gaps, dividido por 3 */
    @media (min-width: 1024px) {
        .card {
            flex: 0 1 calc((100% - 2 * var(--space4)) / 3);
        }
    }
  `;

    private getCurrentEntity(): User {
        if (this.IsChoosingPlanForOrg) {
            return this.Org;
        }
        return this.User;
    }

    onQuantityChange(e) {
        const value = parseInt(e.target.value);
        this.TeamQuantity = isNaN(value) ? 0 : value;
    }
    onQuantityFocus(e) {
        if (parseInt(e.target.value) === 0) {
            e.target.select();
        }
    }
    render() {
        // Users shold only see these cards when they have no plan yet or when
        // they're on the free plan. For all other cases they should first go
        // to the billing portal to cancel their subscription - which will then
        // put them on a PaymentPlan=None
        const entity = this.getCurrentEntity();
        if (entity.PaymentPlan != "None" &&
            entity.PaymentPlan != "Free"){
            return html`<p>How did you get here?</p>`
        }
        return html`
      <div class="container">
        <h1 style="text-align: center;">${this.getTitleText()}</h1>
        <div class="grid">
            ${this.renderFreePlanCard()}
          <!-- Solo -->
          <div class="card">
            <h2>Solo</h2>
            <p>For individuals</p>
            <div class="price">$${this.SoloPrice}/mo</div>
            <ul>
                <li>✔ 3 users</li>
                <li>✔ Unlimited repositories</li>
                <li>✔ 10 GB storage</li>
                <li>✔ Unlimited CI/CD minutes, priority, 1x concurrency</li>
            </ul>
            <form method="post" action="${SubscribeWithStripeUrl + window.location.search}">
              <input type="hidden" name="${CsrfFormName}" value="${GetCsrfFormValue()}">
              <input type="hidden" name="${StripePriceIdParamName}" 
              .value=${this.StripeSoloPriceId}>
              <input type="hidden" name="${StripeQuantityParamName}" value="1">
              ${this.renderOrgInputs()}
              <button type="submit">${this.getSoloBtnText()}</button>
            </form>
          </div>

          <!-- Teams -->
          <div class="card teams">
            <h2>Teams</h2>
            <p>For collaboration</p>
            <div class="price">$${this.TeamsPrice}/user/mo</div>
            <ul>
              <li>✔ Unlimited users</li>
              <li>✔ Unlimited repositories</li>
              <li>✔ 50 GB storage</li>
              <li>✔ Unlimited CI/CD minutes, priority, 3x concurrency</li>
            </ul>
            <form method="post" action="${SubscribeWithStripeUrl + window.location.search}">
                <input type="hidden" name="${CsrfFormName}" value="${GetCsrfFormValue()}">
                <input type="hidden" name="${StripePriceIdParamName}" 
                .value=${this.StripeTeamPriceId}>

                <label class="quantity-label">
                    Team size:
                    <input 
                    class="quantity-input"
                    type="number" 
                    name="${StripeQuantityParamName}"
                    @input=${this.onQuantityChange}
                    @focus=${this.onQuantityFocus}
                    value=${this.TeamQuantity}>
                </label>

                ${this.renderOrgInputs()}

                <button type="submit" ?disabled=${this.TeamQuantity < 1}>
                    ${this.getTeamBtnText()}
                </button>
            </form>
          </div>
        </div>
      </div>
    `;
    }

    private renderFreePlanCard(){
        const entity = this.getCurrentEntity();
        // The free plan card only appears if the user has no plan yet.
        if (entity.PaymentPlan == "None"){
            return html`
            <div class="card free">
            <h2>Free</h2>
            <p>For getting started</p>
            <div class="price">$0/mo</div>
            <ul>
                <li>✔ 1 user</li>
                <li>✔ 1 repository</li>
                <li>✔ 250 MB storage</li>
                <li>✔ Unlimited CI/CD minutes</li>
            </ul>
            <form  method="post" action="${SubscribeTrialUrl}">
                <input type="hidden" name="${CsrfFormName}" value="${GetCsrfFormValue()}">
                ${this.renderOrgInputs()}
                <button type="submit">Get Free</button>
            </form>
            </div>
            `
        }
        return html``
    }

    private getTitleText(){
        if (this.IsChoosingPlanForOrg){
            if (this.Org.PaymentPlan == "None") {
                return `Choose a plan for ${this.Org.Username}`
            }
            return `Upgrade plan of ${this.Org.Username}`
        }else{
            if (this.User.PaymentPlan == "None"){
                return "Choose your plan"
            }
            return "Upgrade your plan"
        }
    }
    private getSoloBtnText(){
        const entity = this.getCurrentEntity();

        if (entity.PaymentPlan == "None"){
            return "Get Solo"
        }
        return "Upgrade to Solo"
    }
    private getTeamBtnText(){
        const entity = this.getCurrentEntity();

        if (entity.PaymentPlan == "None"){
            return "Get Team"
        }
        return "Upgrade to Team"
    }
    private renderOrgInputs() {
        if (this.IsChoosingPlanForOrg){
            return html`
                <input type="hidden" name="${IsChoosingPlanForOrgParamName}" value="${this.IsChoosingPlanForOrg}">
                <input type="hidden" name="${OrganizationNameParamName}" value=${this.Org!.Username}>
            `;
        }
        return html`
            <input type="hidden" name="${IsChoosingPlanForOrgParamName}" value="${false}">
        `;
    }
}

customElements.define("plans-cards", PlansCards);

declare global {
    interface HTMLElementTagNameMap {
        'plans-cards': PlansCards;
    }
}