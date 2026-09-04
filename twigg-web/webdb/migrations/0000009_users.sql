CREATE TABLE IF NOT EXISTS users2(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    email      TEXT NOT NULL,
    state INTEGER NOT NULL,
    isOrganization BOOLEAN NOT NULL DEFAULT FALSE,

    stripeId   TEXT NOT NULL DEFAULT '',
    cliKeyHash TEXT NOT NULL DEFAULT '',
        
    username   TEXT NOT NULL DEFAULT '',
    passwordHash TEXT NOT NULL DEFAULT '',

    selfPaidSubscription INTEGER NOT NULL,
    selfPaidSubscriptionQuantity INTEGER NOT NULL,
    selfPaidSubscriptionSeatsInUse INTEGER NOT NULL,  /*DEPRECATED - NO LONGER USED*/

    stripeSessionId TEXT NOT NULL DEFAULT '',
    stripeSessionUrl TEXT NOT NULL DEFAULT '',
    stripeSessionPriceId TEXT NOT NULL DEFAULT '',
    stripeSessionQuantity INTEGER NOT NULL DEFAULT 0,
    stripeSubscriptionID TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS user_by_username ON users2 (username);
CREATE INDEX IF NOT EXISTS user_by_email ON users2 (email);
CREATE INDEX IF NOT EXISTS user_by_stripeId ON users2 (stripeId);
CREATE INDEX IF NOT EXISTS user_by_cliKeyHash ON users2 (cliKeyHash);

CREATE TABLE IF NOT EXISTS stripe_subscriptions2(
    stripeSubscriptionId TEXT PRIMARY KEY,
    userId INTEGER NOT NULL,
    isActive BOOLEAN NOT NULL
);