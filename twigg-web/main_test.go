package main

import (
	"encoding/json"
	"fmt"
	"io"
	"monorepo/squeue"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/runners"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-web/cicdqueue"
	"monorepo/twigg-web/handlers/commit"
	"monorepo/twigg-web/handlers/jobshandler"
	"monorepo/twigg-web/handlers/notifications"
	"monorepo/twigg-web/handlers/usereducation"
	"monorepo/twigg-web/job"
	"monorepo/twigg-web/metrics"
	"monorepo/twigg-web/repo"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/server"
	"monorepo/twigg-web/services/cicdpublisher"
	reposervice "monorepo/twigg-web/services/repo"
	"monorepo/twigg-web/services/sign"
	"monorepo/twigg-web/services/twiggtoken"
	userservice "monorepo/twigg-web/services/user"
	"monorepo/twigg-web/srvconfig"
	twiggwebclient "monorepo/twigg-web/twigg-web-client"
	"monorepo/twigg-web/webcomponents"
	"monorepo/twigg/cli"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestHealthAndLandingPage(t *testing.T) {
	srv := GetMockServer(t)
	bowser := NewTestBrowser(srv.C.PublicUrl, t)

	bowser.Get("/")
	bowser.CheckLastResponseWasHtml()
	bowser.Get("/favicon.ico")
	bowser.CheckLastResponseWasNotHtml()
	bowser.Get("/logo.png")
	bowser.CheckLastResponseWasNotHtml()
	bowser.Get("/health")
}

func TestRateLimiter(t *testing.T) {
	// Use a very low QPS and a burst size 2 to allow only 2 to go through
	const maxQps = 0.001
	const maxQpsBurst = 2
	srv := GetMockServer(t, WithRateLimit(maxQps, maxQpsBurst))
	b := NewTestBrowser(srv.C.PublicUrl, t)
	gotRateLimitedCount := 0
	for i := 0; i < 5; i++ {
		resp, err := b.c.Get(b.serverUrl + "/")
		if err != nil {
			b.t.Fatalf("get failed: %s", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK &&
			resp.StatusCode != http.StatusTooManyRequests {
			b.t.Fatalf("unexpected status: %d", resp.StatusCode)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			gotRateLimitedCount += 1
		}
	}
	if gotRateLimitedCount != 3 {
		t.Fatalf("unexpected rate limits: %d", gotRateLimitedCount)
	}
}

func TestRedirectToLoginIfNotAuth(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	b.Get("/home")
	b.CheckCurrentPath("/login")
}
func TestLoginRedirectToHomeIfAlreadyAuth(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get("/login")
	b.CheckCurrentPath("/home")

	b.Post("/logout", nil)
	b.CheckCurrentPath("/login")
}

func TestPasswordLogin(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	b.Get("/home")
	b.CheckCurrentPath("/login")

	b.Post("/login", map[string]string{
		routes.LogInEmailFieldName:    "aang@twigg.vc",
		routes.LogInPasswordFieldName: "wrong password",
	})
	b.CheckCurrentPath("/login")

	b.Post("/login", map[string]string{
		routes.LogInEmailFieldName:    "aang@twigg.vc",
		routes.LogInPasswordFieldName: "yipyip",
	})
	// Users in active plans are redirected to home
	b.CheckCurrentPath("/home")
	b.CheckLastResponseWasHtml()
}

func TestAdmin(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Non admins cant access the dash
	b2 := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b2, "non-admin@twigg.vc")
	b2.CheckGetErrors(routes.AdminDash, http.StatusUnauthorized)
	b2.CheckGetErrors("/admindash/metric/ts/requests", http.StatusUnauthorized)

	// Only admins can
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.AdminDash)
	b.CheckCurrentPath(routes.AdminDash)
	b.CheckCurrentPageContains("admin-dash")

	// Test getting the metrics
	b.Get("/admindash/metric/ts/requests")
	var ts []metrics.TimeSeriesPoint
	err := json.Unmarshal(b.lastResponse, &ts)
	if err != nil {
		t.Fatal(err)
	}
	b.Get("/admindash/metric/ts/requests-millis")
	var fTs []metrics.FloatTimeSeriesPoint
	err = json.Unmarshal(b.lastResponse, &fTs)
	if err != nil {
		t.Fatal(err)
	}

}

func TestUserSettingsPage(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get("/user-settings")
	b.CheckLastResponseWasHtml()
	// Expect user-settings
	b.CheckCurrentPageContains("<user-settings")
	b.CheckCurrentPageContains("aang")
}

func TestGetUserEducationAndPutWelcomeWasShown(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserEducation)
	var resp usereducation.GetUserEducationResponse
	err := json.Unmarshal(b.lastResponse, &resp)
	if err != nil {
		t.Fatal(err)
	}
	if resp.WelcomeWasShown {
		t.Fatalf("expected WelcomeWasShown == false for a user with no education state")
	}

	b.Put(routes.UserEducationWelcomeWasShown, nil, http.StatusOK)

	b.Get(routes.UserEducation)
	err = json.Unmarshal(b.lastResponse, &resp)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.WelcomeWasShown {
		t.Fatalf("expected WelcomeWasShown == true after the put")
	}

	// Putting again is idempotent
	b.Put(routes.UserEducationWelcomeWasShown, nil, http.StatusOK)

	b.Get(routes.UserEducation)
	err = json.Unmarshal(b.lastResponse, &resp)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.WelcomeWasShown {
		t.Fatalf("expected WelcomeWasShown == true after the second put")
	}
}

func TestRepoSettingsPage(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get("/aang/BookOne/settings")
	b.CheckLastResponseWasHtml()
	b.CheckCurrentPageContains(
		"aang",
		"katara", "</repo-settings>", "</twigg-app>", "</body>")
}

func TestRepoSettingsPage_PostPutAndDeleteSecret(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	// Post
	b.Post("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  "satoshi-private-key",
		routes.RepoSecretValueParamName: "we will never know",
	})
	b.Get("/aang/BookOne/settings")
	b.CheckCurrentPageContains("satoshi-private-key")
	b.CheckCurrentPageNotContains("we will never know")
	// Post with already existing secret name
	b.CheckPostErrors("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  "satoshi-private-key",
		routes.RepoSecretValueParamName: "or will we?",
	})
	b.Get("/aang/BookOne/settings")
	b.CheckCurrentPageContains("satoshi-private-key")

	// Put
	b.Put("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  "satoshi-private-key",
		routes.RepoSecretValueParamName: "i don't know",
	}, http.StatusOK)
	b.Get("/aang/BookOne/settings")
	b.CheckCurrentPageContains("satoshi-private-key")
	b.CheckCurrentPageNotContains("i don't know")

	// Delete
	b.Delete("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName: "satoshi-private-key",
	}, http.StatusOK)
	b.Get("/aang/BookOne/settings")
	b.CheckCurrentPageNotContains("satoshi-private-key")
}

func TestRepoSettingsPage_ValidateReservedSecrets(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	// Post
	b.CheckPostErrors("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  reposervice.GitMirrorUrlSecretName,
		routes.RepoSecretValueParamName: "should return error for reserved secret",
	})
	// Put
	b.Put("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  reposervice.GitMirrorUrlSecretName,
		routes.RepoSecretValueParamName: "i don't know",
	}, http.StatusBadRequest)

	// Delete
	b.Delete("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName: reposervice.GitMirrorUrlSecretName,
	}, http.StatusBadRequest)
}

func TestOAuthRegistration(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthRegistration(srv, b, "newuser@gmail.com", "username")
	b.CheckCurrentPath(routes.Home)
	if !strings.Contains(string(b.lastResponse), "PaymentPlan") ||
		!strings.Contains(string(b.lastResponse), "Free") {
		t.Fatalf("expected user to be in free plan")
	}

	// Check the demo repo has been created
	b.CheckCurrentPageContains(strings.ToLower(repo.DemoRepoName))
	b.Get("/username/" + repo.DemoRepoName)
}

func TestTrialAccountSignupAndPermissions(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Register new user should have trial plan
	MockUserOAuthRegistration(srv, b, "newuser@gmail.com", "username")
	// Cant sub again
	b.CheckPostErrors(routes.SubscribeTrialUrl, map[string]string{})

	// The demo repo created on registration consumes one of the two
	// trial repo slots, so only one more repo can be created
	b.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "my-repo",
		routes.NewRepoDescriptionParameterName: "my repo description",
	})
	b.CheckPostErrors(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "my-second-repo",
		routes.NewRepoDescriptionParameterName: "i cant create this one",
	})
	// Can create another one after archiving
	b.Post("/username/my-repo/settings/archive", map[string]string{})
	b.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "my-other-repo",
		routes.NewRepoDescriptionParameterName: "ok create this one",
	})

	// Can create key
	b.Post(routes.GenerateCLIKey, nil)
}

func TestUpgradeToPaidPlan(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Register new user should have trial plan
	MockUserOAuthRegistration(srv, b, "newuser@gmail.com", "username")
	// Choose to upgrade paying via stripe
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	// Dont pay yet and try going to home. Should be redirected to plans.
	b.Get(routes.Home)
	b.CheckCurrentPath(routes.PlansPage)
	_, sId, _ := srv.StripeClientMock.GetLastStripeSession()
	// User gives up and choses trial instead. Stripe session should be canceled
	b.Post(routes.SubscribeTrialUrl, map[string]string{})
	b.CheckCurrentPath(routes.Home)
	srv.StripeClientMock.CheckSessionWasExpired(sId, t)
	// Try again but actually pay this time
	b.Get(routes.PlansPage)
	b.CheckCurrentPath(routes.PlansPage)
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebHook*/ true)
	b.Get(routes.Home)
	b.CheckCurrentPath(routes.Home)
	if !strings.Contains(string(b.lastResponse), "PaymentPlan") ||
		!strings.Contains(string(b.lastResponse), "Solo") {
		t.Fatalf("expected user to be in Solo plan")
	}
}

func TestStripeParamsValidation(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, b, "newuser@gmail.com", "username")
	b.CheckPostErrors(
		routes.SubscribeWithStripeUrl,
		map[string]string{
			routes.StripePriceIdParamName:  "non existing stripe plan",
			routes.StripeQuantityParamName: "1",
		})
	b.CheckPostErrors(
		routes.SubscribeWithStripeUrl,
		map[string]string{
			routes.StripePriceIdParamName:  string(srv.StripeClientMock.GetLatestSoloPriceId()),
			routes.StripeQuantityParamName: "not a number",
		})
}

func TestSuccessfullPaymentWithStripe(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// New user tries to access /home and is redirected to login.
	b.Get("/home")
	b.CheckCurrentPath("/login")
	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")

	// Mock the user starts to pay via stripe for the solo plan
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)

	// Mock the user finished paying in stripe
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebhookEvent*/ true)

	// Now the user can successfully go to /home
	b.Get("/home")
	b.CheckCurrentPath("/home")
	b.CheckLastResponseWasHtml()
}

func TestSuccessfullPaymentWithStripeButMissedWebhook(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")

	// Mock the user starts to pay via stripe for the solo plan
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)

	// Mock the user finished paying in stripe, but that the
	// webhook event was not sent; to simulate the case that we missed
	// processing the webhook event
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebhookEvent*/ false)

	// When the user tries to go to the home page, ther server will ask stripe
	// and realize the session was paid but the webhook was not processed.
	// Thus, the user will sucessfully reach the home page and won't be
	// redirected to choose a payment plan.
	b.Get("/home")
	b.CheckCurrentPath("/home")
	b.CheckLastResponseWasHtml()

	// Its ok if now the webhook is posted. Make a post request to mock
	// stripe posting the event
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)
}

func TestUpdateSubscriptionQuantity_upgradeQuantity(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Momo creates an account, pays for the team plan
	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestTeamPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestTeamPriceId(), 1,
		/*postWebhookEvent*/ true)
	b.Get(routes.UserSettings)
	momoSubId, momoStripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()

	// Momo upgrades subscription quantity
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(momoSubId, momoStripeCustomerId, 2)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)
	b.Get(routes.UserSettings)
}

func TestUpdateSubscriptionQuantity_downgradeQuantity(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Momo creates an account, pays for the team plan
	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestTeamPriceId(), 2)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestTeamPriceId(), 2,
		/*postWebhookEvent*/ true)
	momoSubId, momoStripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()

	// Momo downgrades subscription quantity
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(momoSubId, momoStripeCustomerId, 1)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)
	b.Get(routes.UserSettings)
}

func TestUpdateSubscriptionQuantity_idempotency(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Momo creates an account, pays for the team
	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestTeamPriceId(), 2)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestTeamPriceId(), 2,
		/*postWebhookEvent*/ true)
	momoSubId, momoStripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()

	// Make Momo upgrades but webhook is fired twice with same event
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(momoSubId, momoStripeCustomerId, 1)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(momoSubId, momoStripeCustomerId, 1)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)
}

func TestOrganization_FlowCheck(t *testing.T) {
	srv := GetMockServer(t)
	// momo registers
	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")
	// aang login and create org
	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "air-nomads"

	// aang create org
	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})

	// aang pays a plan for org so it can add members/owners
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestSoloPriceId(), 1 /*postWebhookEvent*/, true)

	// Org should appear in Organizations page for aang
	bAang.Get(routes.OrganizationsPattern)
	if !strings.Contains(string(bAang.lastResponse), orgName) {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// aang cant remove itself because he is the only Owners
	bAang.CheckPostErrors("/orgs/v1/org/air-nomads/remove", map[string]string{
		routes.UsernameParameterName: "aang",
		routes.PermissionParamName:   "3",
	})
	if string(bAang.lastResponse) != "organization needs a least one owner\n" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// aang grants momo Owner permission
	bAang.Post("/orgs/v1/org/air-nomads/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "3",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// Now aang can remove itself because there is another Owner
	bAang.Post("/orgs/v1/org/air-nomads/remove", map[string]string{
		routes.UsernameParameterName: "aang",
		routes.PermissionParamName:   "3",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}
}
func TestOrganization_MemeberCanNotRevokePermisson(t *testing.T) {
	srv := GetMockServer(t)
	// momo registers
	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")
	// aang login and create org
	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "air-nomads"

	// aang create org
	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})

	// aang pays a plan for org so it can add members/owners
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestSoloPriceId(), 1 /*postWebhookEvent*/, true)

	// aang grants momo Member permission
	bAang.Post("/orgs/v1/org/air-nomads/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// momo can not remove aang because momo is not a owner
	bMomo.CheckPostErrors("/orgs/v1/org/air-nomads/remove", map[string]string{
		routes.UsernameParameterName: "aang",
		routes.PermissionParamName:   "3",
	})
	if string(bMomo.lastResponse) != "user does not have owner permission\n" {
		t.Fatalf("unexpected response: %q", string(bMomo.lastResponse))
	}
}

func TestOrganization_MemeberCanNotSubscripbeToTrial(t *testing.T) {
	srv := GetMockServer(t)
	// momo registers
	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")
	// aang login and create org
	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "air-nomads"

	// aang create org
	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})

	// aang pays a plan for org so it can add members/owners
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestSoloPriceId(), 1 /*postWebhookEvent*/, true)

	// aang grants momo Member permission
	bAang.Post("/orgs/v1/org/air-nomads/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// momo cant susbcribe org to trial momo is not a owner
	bMomo.CheckPostErrors(routes.SubscribeTrialUrl, map[string]string{
		routes.IsChoosingPlanForOrgParamName: "true",
		routes.OrganizationNameParamName:     orgName,
	})

	if string(bMomo.lastResponse) != "not allowed\n" {
		t.Fatalf("unexpected response: %q", string(bMomo.lastResponse))
	}
}
func TestOrganization_SubscripbeWithStipe(t *testing.T) {
	srv := GetMockServer(t)
	// momo registers
	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")
	// aang login and create org
	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "air-nomads"

	// aang create org
	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})

	soloPriceId := srv.StripeClientMock.GetLatestSoloPriceId()

	// momo (no role in org) can't create a stripe checkout session
	bMomo.CheckPostErrors(routes.SubscribeWithStripeUrl, map[string]string{
		routes.IsChoosingPlanForOrgParamName: "true",
		routes.OrganizationNameParamName:     orgName,
		routes.StripePriceIdParamName:        string(soloPriceId),
		routes.StripeQuantityParamName:       "1",
	})
	if string(bMomo.lastResponse) != "User does not have owner or member permission\n" {
		t.Fatalf("unexpected response: %q", string(bMomo.lastResponse))
	}

	// aang (owner) can create a checkout session — org has no plan yet
	bAang.Post(routes.SubscribeWithStripeUrl, map[string]string{
		routes.IsChoosingPlanForOrgParamName: "true",
		routes.OrganizationNameParamName:     orgName,
		routes.StripePriceIdParamName:        string(soloPriceId),
		routes.StripeQuantityParamName:       "1",
	})

	// complete the payment so org has a plan
	MockUserFinishedPayingInStripe(srv, bAang, soloPriceId, 1, true)

	// now org has a plan — add momo as member
	bAang.Post("/orgs/v1/org/air-nomads/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// momo (member, not owner) also can't subscribe for the org
	bMomo.CheckPostErrors(routes.SubscribeWithStripeUrl, map[string]string{
		routes.IsChoosingPlanForOrgParamName: "true",
		routes.OrganizationNameParamName:     orgName,
		routes.StripePriceIdParamName:        string(soloPriceId),
		routes.StripeQuantityParamName:       "1",
	})
	if string(bMomo.lastResponse) != "not allowed\n" {
		t.Fatalf("unexpected response: %q", string(bMomo.lastResponse))
	}
}

// Mirrors TestSuccessfullPaymentWithStripe but for orgs. Newly-created orgs
// start on the trial plan, so a member cannot be added until the org upgrades
// to a plan with more seats.
func TestOrganization_SuccessfulPaymentWithStripe(t *testing.T) {
	srv := GetMockServer(t)

	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "fire-nation"

	// Create org - it starts on trial
	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})
	bAang.CheckPostErrors(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})

	// Org appears in aang's organizations list
	bAang.Get(routes.OrganizationsPattern)
	bAang.CheckCurrentPageContains(orgName)

	// Org does not appears in momo's organizations list
	bMomo.Get(routes.OrganizationsPattern)
	bMomo.CheckCurrentPageNotContains(orgName)

	// Aang pay for org via stripe (Solo plan)
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestSoloPriceId(), 1, true)

	// Org page is accessible and adding a member now succeeds
	bAang.Get(routes.PathToOrganization(orgName))
	bAang.CheckLastResponseWasHtml()
	bAang.CheckCurrentPageContains(orgName)

	bAang.Post(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4", // Member param
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// Now Org appears in momo's organizations list
	bMomo.Get(routes.OrganizationsPattern)
	bMomo.CheckCurrentPageContains(orgName)
}

func TestOrganization_SuccessfulPaymentWithStripeButMissedWebhook(t *testing.T) {
	srv := GetMockServer(t)
	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "earth-kingdom"

	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})

	// Pay but do NOT post the webhook event
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestSoloPriceId(), 1, false)

	// Visiting the org page triggers a lazy subscription update - page accessible
	bAang.Get(routes.PathToOrganization(orgName))
	bAang.CheckLastResponseWasHtml()
	bAang.CheckCurrentPageContains(orgName)

	// Posting the webhook afterwards is also fine (idempotent)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, t)
}

// Verifies that when an org's seat count is upgraded the org can immediately
// add new members.
func TestOrganization_UpdateSubscriptionQuantity_UpgradeQuantity(t *testing.T) {
	srv := GetMockServer(t)

	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")

	bBuko := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bBuko, "buko@twigg.vc", "buko")

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "earth-rumble"

	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})
	// Pays for 2 seats
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestTeamPriceId(), 2)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestTeamPriceId(), 2, true)
	orgSubId, orgStripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()

	// Add momo making org reach max seats available (2/2, aang and momo)
	bAang.Post(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response adding momo: %q", string(bAang.lastResponse))
	}
	// Org appears in momo's organizations list
	bMomo.Get(routes.OrganizationsPattern)
	bMomo.CheckCurrentPageContains(orgName)

	// Cannot add buko, total would be 3, but org only have 2
	bAang.CheckPostErrors(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "buko",
		routes.PermissionParamName:   "4",
	})

	// Upgrade to qty=3 via stripe subscription update webhook
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(orgSubId, orgStripeCustomerId, 3)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, t)

	// Can now add buko
	bAang.Post(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "buko",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response adding buko after upgrade: %q", string(bAang.lastResponse))
	}

	// Org appears in buko's organizations list
	bBuko.Get(routes.OrganizationsPattern)
	bBuko.CheckCurrentPageContains(orgName)
}

// Verifies that when seat count drops, excess members are evicted.
// qty=3 lets aang add momo+buko. Downgrading to qty=2
// evicts one, so a subsequent add of a third member fails.
func TestOrganization_UpdateSubscriptionQuantity_DowngradeQuantity(t *testing.T) {
	srv := GetMockServer(t)

	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")

	bBuko := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bBuko, "buko@twigg.vc", "buko")

	bZuko := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bZuko, "zuko@fire.nation", "zuko")

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "fire-sages"

	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestTeamPriceId(), 3)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestTeamPriceId(), 3, true)
	orgSubId, orgStripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()

	// Add momo and buko
	bAang.Post(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response adding momo: %q", string(bAang.lastResponse))
	}
	bAang.Post(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "buko",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response adding buko: %q", string(bAang.lastResponse))
	}

	// Downgrade to qty=2: one member is evicted
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(orgSubId, orgStripeCustomerId, 2)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, t)

	bAang.Get(routes.PathToOrganization(orgName))
	bAang.CheckCurrentPageContains("aang")
	respString := strings.ToLower(string(bAang.lastResponse))
	momoPresent := strings.Contains(respString, "momo")
	bukoPresent := strings.Contains(respString, "buko")
	if momoPresent == bukoPresent {
		t.Fatalf("expected exactly one of momo/buko to be evicted, got momo=%v buko=%v", momoPresent, bukoPresent)
	}

	// Adding zuko fails: no seats leaft
	bAang.CheckPostErrors(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "zuko",
		routes.PermissionParamName:   "4",
	})
}

func TestOrganization_UpdateSubscriptionQuantity_Idempotency(t *testing.T) {
	srv := GetMockServer(t)

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "white-lotus"

	bAang.Post(routes.NewOrganizationPattern, map[string]string{
		routes.NewOrganizationNameParamName: orgName,
	})
	MockUserChoosesToPayForOrgViaStripe(srv, bAang, orgName, srv.StripeClientMock.GetLatestTeamPriceId(), 2)
	MockUserFinishedPayingInStripe(srv, bAang, srv.StripeClientMock.GetLatestTeamPriceId(), 2, true)
	orgSubId, orgStripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()

	// Same subscription update posted twice should not error
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(orgSubId, orgStripeCustomerId, 2)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, t)
	srv.StripeClientMock.MockSubscriptionUpdatedQuantity(orgSubId, orgStripeCustomerId, 2)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, t)
}

func TestOrganization_OrgPage(t *testing.T) {
	srv := GetMockServer(t)

	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "air-temple"

	MockSetupOrgAndPay(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)

	// Add momo as member
	bAang.Post(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response: %q", string(bAang.lastResponse))
	}

	// Create an org repo
	bAang.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "temple-scrolls",
		routes.NewRepoDescriptionParameterName: "air temple scrolls",
		routes.OrganizationNameParamName:       orgName,
	})

	// Org page shows org name, owner (aang), member (momo) and repo
	bAang.Get(routes.PathToOrganization(orgName))
	bAang.CheckLastResponseWasHtml()
	bAang.CheckCurrentPageContains(orgName)
	bAang.CheckCurrentPageContains("aang")
	bAang.CheckCurrentPageContains("momo")
	bAang.CheckCurrentPageContains("temple-scrolls")

	// Members can also view the org page
	bMomo.Get(routes.PathToOrganization(orgName))
	bMomo.CheckLastResponseWasHtml()
	bMomo.CheckCurrentPageContains(orgName)
}

func TestOrganization_CreateOrgRepo(t *testing.T) {
	srv := GetMockServer(t)

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "sun-warriors"

	MockSetupOrgAndPay(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)

	// Create a repo under the org
	bAang.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "ancient-forms",
		routes.NewRepoDescriptionParameterName: "ancient fire bending forms",
		routes.OrganizationNameParamName:       orgName,
	})

	// Repo shows on org page
	bAang.Get(routes.PathToOrganization(orgName))
	bAang.CheckCurrentPageContains("ancient-forms")

	// Repo is accessible at /orgname/reponame
	bAang.Get("/" + orgName + "/ancient-forms")
	bAang.CheckLastResponseWasHtml()

	// Duplicate repo name is rejected
	bAang.CheckPostErrors(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "ancient-forms",
		routes.NewRepoDescriptionParameterName: "duplicate",
		routes.OrganizationNameParamName:       orgName,
	})
}

// Verifies the complete CLI authentication flow for organization repositories.
// A user cannot push without an organization membership, can push after
// being added as a member, and is blocked again after the membership
// is removed.
func TestOrganization_MemberGetsWriteAccessViaCliPush(t *testing.T) {
	srv := GetMockServer(t)

	// momo registers
	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@twigg.vc", "momo")

	bMomo.Post(routes.GenerateCLIKey, nil)
	momoKey := srv.KeysMock.GetLastRandomCliKey()

	// aang creates the org, pays for it, and creates an org repo
	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	orgName := "kyoshi-warriors"
	MockSetupOrgAndPay(srv, bAang, orgName, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	bAang.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "warrior-training",
		routes.NewRepoDescriptionParameterName: "kyoshi warrior training repo",
		routes.OrganizationNameParamName:       orgName,
	})

	// Set up momo's CLI pointing at the org repo
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", orgName+"/warrior-training")
	tw.Run("key", momoKey)

	// momo can't push yet - not an org member
	tw.WriteFile("scroll.txt", "earth scroll")
	tw.Run("commit", "add scroll")
	tw.Run("push")
	tw.CheckOutContains("permission denied")

	// aang adds momo as an org member - momo gets write access to org repos
	bAang.Post(routes.PathToOrganization(orgName)+"/add", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response adding momo: %q", string(bAang.lastResponse))
	}

	// momo can now push
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// aang removes momo from the org → write access revoked
	bAang.Post(routes.PathToOrganization(orgName)+"/remove", map[string]string{
		routes.UsernameParameterName: "momo",
		routes.PermissionParamName:   "4",
	})
	if string(bAang.lastResponse) != "ok" {
		t.Fatalf("unexpected response removing momo: %q", string(bAang.lastResponse))
	}

	// momo can no longer pull
	tw.Run("pull")
	tw.CheckOutContains("permission denied")
}

func TestTrialCantCreateMoreThenTwoRepos(t *testing.T) {
	srv := GetMockServer(t)

	// katara can only create two repos bc they're on the trial plan
	b1 := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b1, "katara@twigg.vc")
	b1.CheckCurrentPath("/home")
	b1.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "ALLOWED-REPO",
		routes.NewRepoDescriptionParameterName: "allowed",
	})
	b1.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "ALLOWED-REPO-2",
		routes.NewRepoDescriptionParameterName: "also allowed",
	})
	b1.CheckPostErrors(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "NOT-ALLOWED-REPO",
		routes.NewRepoDescriptionParameterName: "not allowed",
	})

	// Aang should be able to create since they're on a team plan
	b2 := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b2, "aang@twigg.vc")
	b2.CheckCurrentPath("/home")
	b2.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "my-new-repo",
		routes.NewRepoDescriptionParameterName: "my repo description",
	})
	// Expect no redirect
	b2.CheckCurrentPath(routes.NewRepoUrl)
}

func TestCantDuplicateRepoName(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "duplicate",
		routes.NewRepoDescriptionParameterName: "my repo description",
	})
	b.CheckPostErrors(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "duplicate",
		routes.NewRepoDescriptionParameterName: "my repo description",
	})
}

func TestArchiveRepo(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	// Archive the BookTwo repo
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Post("/aang/BookTwo/settings/archive", map[string]string{})
	if string(b.lastResponse) != "ok" {
		t.Fatalf("got %s", b.lastResponse)
	}
	// Repo doesn't show up on users to who the repo is shared
	b2 := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b2, "katara@twigg.vc")

	b2.CheckCurrentPageNotContains("BookTwo")
}
func TestArchiveRepo_NotOwner(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Not owner should not be allowed to archive repo.
	MockUserOAuthSignIn(srv, b, "katara@twigg.vc")
	b.CheckPostErrors("/aang/BookTwo/settings/archive", map[string]string{})
}

func TestLoadMoreSubmittedCommits(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get("/aang/BookTwo/load-s?starting-at=0")
	var c []webcomponents.FrontendCommit
	err := json.Unmarshal(b.lastResponse, &c)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 1 {
		t.Fatalf("got len %d", len(c))
	}
}

func TestSetGitMirror(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Post("/aang/BookTwo/settings/git-mirror-url", map[string]string{
		"url": "https://secret@useless-git-server.com/my/repo.git",
	})
	if string(b.lastResponse) != "ok" {
		t.Fatalf("got %s", b.lastResponse)
	}
}

func TestBinDownloads(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	b.Get("/bin/tw_check")
	lastRespString := string(b.lastResponse)
	if lastRespString != "fake binary for tests" {
		t.Fatal("unexpected check result")
	}
}

func TestTwiggIntegration(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Do full account registration
	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebhookEvent*/ true)

	// Create a CLI key
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())

	tw.Run("pull")
	tw.CheckOutContains("permission denied")

	b2 := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b2, "aang@twigg.vc")
	MockUserGrantsPermission(srv, b2, "aang", "BookOne", "momo")

	tw.Run("pull")
	tw.CheckOutContains("up to date")

	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	MockUserRevokePermission(srv, b2, "aang", "BookOne", "momo")
	tw.Run("pull")
	tw.CheckOutContains("permission denied")
}

func TestRepoPageAndSubmitFlow(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	b.Get("/aang/BookOne")
	b.CheckCurrentPageContains("create a.txt")
	b.Post("/aang/BookOne/c/1/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/1/submit", map[string]string{
		"version": "0",
	})
	b.CheckCurrentPageContains("create a.txt")
}

func TestRemoveLgtmFailsAfterSubmit(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push a commit
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Adding then removing the LGTM before submit works fine
	b.Post("/aang/BookOne/c/1/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/1/r-lgtm", map[string]string{})

	// LGTM again and submit the commit
	b.Post("/aang/BookOne/c/1/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/1/submit", map[string]string{
		"version": "0",
	})

	// Removing the LGTM from an already-submitted commit must fail
	b.CheckPostErrors("/aang/BookOne/c/1/r-lgtm", map[string]string{})
}

func TestSetNextServerId(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push a commit
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Set the next server id. The next pushed commit must get it.
	tw.Run("set-server-id", "100")
	tw.CheckOutContains("next server id set to 100")
	tw.WriteFile("b.txt", "bbb")
	tw.Run("commit", "create b.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	tw.Run("log")
	tw.CheckOutContains("c/100")
}

func TestRenameCommit(t *testing.T) {

	// Mock server and set user (aang)
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Set up CLI and push a commit
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "hello")
	tw.Run("commit", "original message")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// rename the commit
	b.CheckPostJsonReturnsNoContent("/aang/BookOne/c/1/rename", `{"Message":"renamed message"}`)

	// verify the new message is visible on the commit page
	b.Get("/aang/BookOne/c/1")
	b.CheckLastResponseWasHtml()
	if !strings.Contains(string(b.lastResponse), "renamed message") {
		t.Fatalf("expected renamed message on commit page, got: %s", string(b.lastResponse))
	}

	// rename to empty message must fail
	status := b.PostJson("/aang/BookOne/c/1/rename", struct{ Message string }{Message: ""})
	if status == http.StatusNoContent {
		t.Fatalf("expected error when renaming to empty message, got 204")
	}
}

func TestCreateRollback(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli, push a commit, add LGTM and submit it
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	b.Post("/aang/BookOne/c/1/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/1/submit", map[string]string{
		"version": "0",
	})
	// Create a rollback commit. The response is the url to the new commit
	b.Post("/aang/BookOne/c/1/rollback", map[string]string{})
	if string(b.lastResponse) != "/aang/BookOne/c/2" {
		t.Fatalf("expected resp `/aang/BookOne/c/2` got %v", b.lastResponse)
	}

	// Check that we cant rollback before submit
	b.CheckPostErrors("/aang/BookOne/c/2/rollback", map[string]string{})
	// Ok to rollback after submit
	b.Post("/aang/BookOne/c/2/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/2/submit", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/2/rollback", map[string]string{})
	// Expect the response to be the url to redirect to
	if string(b.lastResponse) != "/aang/BookOne/c/3" {
		t.Fatalf("expected resp `/aang/BookOne/c/3` got %v", b.lastResponse)
	}
}

func TestPullByRepoId(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "1/1")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.Run("pull")
	tw.CheckOutContains("already up to date")
}

func TestCommitPage(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Do full account registration, setup CLI Key, create cli client
	// write a commit and push
	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebhookEvent*/ true)
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())

	tw.WriteFile("a.txt", "v0 a.txt")
	tw.Run("commit", "create a.txt")

	tw.Run("push")
	tw.CheckOutContains("permission denied")

	b2 := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b2, "aang@twigg.vc")
	MockUserGrantsPermission(srv, b2, "aang", "BookOne", "momo")

	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	tw.WriteFile("a.txt", "v1 a.txt")
	tw.Run("amend")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Create a child commit (c/2) on top of c/1
	tw.WriteFile("b.txt", "v0 b.txt")
	tw.Run("commit", "create b.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Check the commit page
	// c/1 has c/2 as child
	b.Get("/aang/BookOne/c/1")
	b.CheckCurrentPath("/aang/BookOne/c/1")
	b.CheckCurrentPageContains(`children="[2]"`)

	// c/2 has no children
	b.Get("/aang/BookOne/c/2")
	b.CheckCurrentPageContains(`children="[]"`)

	// Create another child of c/1 (c/3) and rebase c/2 onto it:
	tw.Run("goto", "1")
	tw.WriteFile("c.txt", "v0 c.txt")
	tw.Run("commit", "create c.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	tw.Run("rebase", "2", "3")
	tw.Run("goto", "2")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// c/1 must only list c/3, and c/3 must list c/2
	b.Get("/aang/BookOne/c/1")
	b.CheckCurrentPageContains(`children="[3]"`)
	b.Get("/aang/BookOne/c/3")
	b.CheckCurrentPageContains(`children="[2]"`)

	b.Get("/aang/BookOne/c/1")

	// Post new comment thread
	b.Post("/aang/BookOne/c/1/new-thread?version=0", map[string]string{
		"comment": "my comment",
	})
	b.CheckLastResponseWasNotHtml()
	var discussionTh commit.FrontendThread
	if err := json.Unmarshal(b.lastResponse, &discussionTh); err != nil {
		t.Fatalf("failed to unmarshal thread: %v body=%q", err, b.lastResponse)
	}
	if discussionTh.Type != "CommentsOnCommitVersion" {
		t.Fatalf("got thread type %q, want CommentsOnCommitVersion",
			discussionTh.Type)
	}
	if discussionTh.Filename != "" || discussionTh.Line != 0 {
		t.Fatalf("got thread on %q line %d, want no file and no line",
			discussionTh.Filename, discussionTh.Line)
	}
	if discussionTh.CreatedOn.IsZero() {
		t.Fatal("thread should have CreatedOn set")
	}

	// Post a comment thread anchored to a line of a file
	b.Post("/aang/BookOne/c/1/new-thread?version=0&file=a.txt&line=1",
		map[string]string{
			"comment": "my inline comment",
		})
	b.CheckLastResponseWasNotHtml()
	var inlineTh commit.FrontendThread
	if err := json.Unmarshal(b.lastResponse, &inlineTh); err != nil {
		t.Fatalf("failed to unmarshal thread: %v body=%q", err, b.lastResponse)
	}
	if inlineTh.Filename != "a.txt" || inlineTh.Line != 1 {
		t.Fatalf("got thread on %q line %d, want a.txt line 1",
			inlineTh.Filename, inlineTh.Line)
	}

	// Read a file content
	b.Get("/aang/BookOne/c/1/blob?file=a.txt")
	if string(b.lastResponse) != "v1 a.txt" {
		t.Fatalf("got blob %s", b.lastResponse)
	}
	b.Get("/aang/BookOne/c/1/blob?file=a.txt&version=0")
	if string(b.lastResponse) != "v0 a.txt" {
		t.Fatalf("got blob %s", b.lastResponse)
	}
}
func TestNotifications(t *testing.T) {
	srv := GetMockServer(t)

	// aang logs in and creates commit
	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")

	bAang.Get(routes.UserSettings)
	bAang.Post(routes.GenerateCLIKey, nil)

	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)

	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())

	// aang create commit and push
	tw.WriteFile("a.txt", "v0 a.txt")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// momo registers AND subscribes
	bMomo := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, bMomo, "momo@northern-temple.air", "momo")

	MockUserChoosesToPayViaStripe(srv, bMomo, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, bMomo, srv.StripeClientMock.GetLatestSoloPriceId(), 1, true)

	// aang grants momo permission
	bAang.Post(
		"/aang/BookOne/settings/add",
		map[string]string{
			routes.UsernameParameterName: "momo",
		},
	)

	// momo comments on aang's commit
	bMomo.Get("/aang/BookOne/c/1")
	bMomo.CheckLastResponseWasHtml()
	bMomo.Post("/aang/BookOne/c/1/new-thread?version=0", map[string]string{
		"comment": "my comment",
	})
	bMomo.CheckLastResponseWasNotHtml()

	// aang should receive notification
	bAang.Get(routes.Notifications)
	bAang.CheckLastResponseWasNotHtml()

	var ns1 notifications.GetNotificationsResponse
	if err := json.Unmarshal(bAang.lastResponse, &ns1); err != nil {
		t.Fatalf("failed to unmarshal notifications: %v body=%q",
			err, string(bAang.lastResponse))
	}
	if len(ns1.Notifications) == 0 {
		t.Fatalf("expected notification for aang, got body=%s",
			string(bAang.lastResponse))
	}
	if ns1.Notifications[0].ReadAt != "" {
		t.Fatalf("expected notification to start unread (ReadAt empty), got ReadAt=%q", ns1.Notifications[0].ReadAt)
	}
	if ns1.Notifications[0].SeenAt != "" {
		t.Fatalf("expected notification to start unseen (SeenAt empty), got SeenAt=%q", ns1.Notifications[0].SeenAt)
	}
	if !strings.Contains(ns1.Notifications[0].Message, "momo commented on c/1") {
		t.Fatalf("expected notification message to contain %q, got %q",
			"momo commented on c/1", ns1.Notifications[0].Message)
	}

	// aang marks notification as seen: SeenAt must become non-empty
	notifId := ns1.Notifications[0].Id
	bAang.CheckPostJsonReturnsNoContent(routes.NotificationMarkSeen, fmt.Sprintf(`{"NotificationIds":[%d]}`, notifId))

	bAang.Get(routes.Notifications)
	bAang.CheckLastResponseWasNotHtml()
	var afterSeen notifications.GetNotificationsResponse
	err := json.Unmarshal(bAang.lastResponse, &afterSeen)
	if err != nil {
		t.Fatalf("failed to unmarshal notifications after MarkSeen: %v body=%q", err, string(bAang.lastResponse))
	}
	if len(afterSeen.Notifications) != 1 {
		t.Fatalf("expected 1 notification after MarkSeen, got %d", len(afterSeen.Notifications))
	}
	if afterSeen.Notifications[0].SeenAt == "" {
		t.Fatalf("expected SeenAt to be non-empty after MarkSeen")
	}
	if afterSeen.Notifications[0].ReadAt != "" {
		t.Fatalf("expected ReadAt to still be empty after MarkSeen, got %q", afterSeen.Notifications[0].ReadAt)
	}

	// aang mark as read (JSON body, returns 204)
	bAang.CheckPostJsonReturnsNoContent(routes.NotificationMarkRead, fmt.Sprintf(`{"NotificationId":%d}`, notifId))

	// GET again and confirm ReadAt is now non-empty for that same notification
	bAang.Get(routes.Notifications)
	bAang.CheckLastResponseWasNotHtml()

	var ns2 notifications.GetNotificationsResponse

	if err := json.Unmarshal(bAang.lastResponse, &ns2); err != nil {
		t.Fatalf("failed to unmarshal notifications(after): %v body=%q",
			err, string(bAang.lastResponse))
	}
	if len(ns2.Notifications) != 1 {
		t.Fatalf("expected exactly 1 notification, got %d body=%s",
			len(ns2.Notifications), string(bAang.lastResponse))
	}
	if !strings.Contains(ns2.Notifications[0].Message, "momo commented on c/1") {
		t.Fatalf("expected notification message to contain %q, got %q",
			"momo commented on c/1", ns2.Notifications[0].Message)
	}
	if ns2.Notifications[0].Id != notifId {
		t.Fatalf("notification id changed: expected %d got %d",
			notifId, ns2.Notifications[0].Id)
	}
	if ns2.Notifications[0].ReadAt == "" {
		t.Fatalf("expected notification to be read (ReadAt non-empty)")
	}
	if ns2.Notifications[0].SeenAt == "" {
		t.Fatalf("expected SeenAt to be non-empty after MarkRead (should be auto-populated)")
	}

	// aang replies to momo's thread (thread id = 1)
	bAang.Post("/aang/BookOne/c/1/thread/1", map[string]string{
		"comment": "reply from aang",
	})
	bAang.CheckLastResponseWasNotHtml()

	// aang should NOT receive another notification
	bAang.Get(routes.Notifications)
	bAang.CheckLastResponseWasNotHtml()
	var aangAfterReply notifications.GetNotificationsResponse
	if err := json.Unmarshal(bAang.lastResponse, &aangAfterReply); err != nil {
		t.Fatalf("failed to unmarshal aang notifications(after reply): %v body=%q",
			err, string(bAang.lastResponse))
	}
	if len(aangAfterReply.Notifications) != 1 {
		t.Fatalf("expected aang to still have exactly 1 notification, got %d",
			len(aangAfterReply.Notifications))
	}

	// momo should receive notification
	bMomo.Get(routes.Notifications)
	bMomo.CheckLastResponseWasNotHtml()
	var momoAfterReply notifications.GetNotificationsResponse
	if err := json.Unmarshal(bMomo.lastResponse, &momoAfterReply); err != nil {
		t.Fatalf("failed to unmarshal momo notifications(after reply): %v body=%q",
			err, string(bMomo.lastResponse))
	}
	if len(momoAfterReply.Notifications) != 1 {
		t.Fatalf("expected momo to receive exactly 1 reply notification, got %d body=%s",
			len(momoAfterReply.Notifications), string(bMomo.lastResponse))
	}
	if !strings.Contains(momoAfterReply.Notifications[0].Message, "aang replied to a comment on c/1") {
		t.Fatalf("expected reply notification message, got %q",
			momoAfterReply.Notifications[0].Message)
	}
	if momoAfterReply.Notifications[0].ReadAt != "" {
		t.Fatalf("expected reply notification to start unread")
	}
	if momoAfterReply.Notifications[0].SeenAt != "" {
		t.Fatalf("expected reply notification to start unseen")
	}

	// momo resolves the thread
	bMomo.Post("/aang/BookOne/c/1/thread/1", map[string]string{
		"comment":  "resolving",
		"resolved": "1",
	})
	bMomo.CheckLastResponseWasNotHtml()

	// momo should NOT receive another notification,
	// momo should only have 1 notification (from Aang's reply)
	bMomo.Get(routes.Notifications)
	bMomo.CheckLastResponseWasNotHtml()
	var momoAfterResolve notifications.GetNotificationsResponse
	if err := json.Unmarshal(bMomo.lastResponse, &momoAfterResolve); err != nil {
		t.Fatalf("failed to unmarshal momo notifications(after resolve): %v body=%q",
			err, string(bMomo.lastResponse))
	}
	if len(momoAfterResolve.Notifications) != 1 {
		t.Fatalf("expected momo to still have exactly 1 notification, got %d",
			len(momoAfterResolve.Notifications))
	}

	// aang should receive notification, that momo 'resolved' thread
	bAang.Get(routes.Notifications)
	bAang.CheckLastResponseWasNotHtml()
	var aangAfterResolve notifications.GetNotificationsResponse
	if err := json.Unmarshal(bAang.lastResponse, &aangAfterResolve); err != nil {
		t.Fatalf("failed to unmarshal aang notifications(after resolve): %v body=%q",
			err, string(bAang.lastResponse))
	}
	if len(aangAfterResolve.Notifications) != 2 {
		t.Fatalf("expected aang to have 2 notifications after resolve, got %d body=%s",
			len(aangAfterResolve.Notifications), string(bAang.lastResponse))
	}
	latest := aangAfterResolve.Notifications[0]
	if !strings.Contains(latest.Message, "momo resolved a comment on c/1") {
		t.Fatalf("expected resolve notification message, got %q",
			latest.Message)
	}
	if latest.ReadAt != "" {
		t.Fatalf("expected resolve notification to start unread")
	}
	if latest.SeenAt != "" {
		t.Fatalf("expected resolve notification to start unseen")
	}
}

func TestQuotaEnforcementOnTwiggPushes(t *testing.T) {
	// Mock a smaller quota for testing
	backup := userservice.TeamStorageQuota
	t.Cleanup(func() {
		userservice.TeamStorageQuota = backup
	})
	userservice.TeamStorageQuota = 5 * 1024 // 5 kB

	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")

	// Create a CLI key and setup the cli
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())

	// Push a small txt. Should be ok.
	tw.WriteFile("a.txt", "aa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckActiveCommit(cli.CheckCommitArg{
		Id:          1,
		Version:     0,
		HasServerId: true,
		HasServerV:  true,
		ServerId:    1,
		ServerV:     0,
	})
	// Try pushing a big commit. Expect push not to succeed due to no quota.
	tw.WriteFile("b.txt", strings.Repeat("R8bT1nWmE4uJ6sC0gF9r", 500))
	tw.Run("commit", "create b.txt")
	tw.Run("push")
	tw.CheckActiveCommit(cli.CheckCommitArg{
		Id:          2,
		Version:     0,
		HasServerId: false,
		HasServerV:  false,
	})
}

func TestCantPushOrPullIfOwnerHasNoSelfPaidSub(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Register, pay via stripe and create a repo
	MockUserOAuthRegistration(srv, b, "momo@northern-temple.air", "momo")
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(
		srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1 /*postWebhookEvent*/, true)
	b.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "momorepo",
		routes.NewRepoDescriptionParameterName: "my repo description",
	})
	b.Post(routes.GenerateCLIKey, nil)
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "momo/momorepo")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())

	// Push a commit
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Mock that the plan was canceled
	subId, stripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()
	srv.StripeClientMock.MockSubscriptionCanceled(subId, stripeCustomerId)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)

	// Next pull should error
	tw.Run("pull")
	tw.CheckOutContains("no subscription")
	tw.WriteFile("a.txt", "AAA")
	tw.Run("commit", "change to AAA")
	tw.Run("push")
	tw.CheckOutContains("no subscription")
}

func TestTwiggDocs(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())

	tw.WriteFile("subfolder/README.md", "# Hello, world!")
	tw.Run("commit", "create doc")
	tw.Run("push")

	b.Get("/aang/BookOne/docs")
	b.CheckLastResponseWasHtml()
	b.Get("/aang/BookOne/docs/subfolder/README.md?c=1")
	if string(b.lastResponse) != "# Hello, world!" {
		t.Fatalf("bad resp %s", b.lastResponse)
	}
}

func TestMaintenanceServer(t *testing.T) {
	port, storageFolder, _, _ := setupTest(
		/*startTrackServer*/ false,
		/*useMockTrackServer*/ false, t)
	setProdConfigEnvVars(t)
	cfg := srvconfig.ProdConfig(port, storageFolder, "")
	srv := server.NewSrv(cfg)
	const runInMaintenanceMode = true
	go srv.Run(runInMaintenanceMode)
	// Wait for server to be running. The maintenance-server has the goal
	// of having the code as simple as possible, so the isReady
	// flag can't be used to know if it started or not
	time.Sleep(time.Millisecond * 100)

	c := &http.Client{}
	resp, err := c.Get(fmt.Sprintf("http://localhost:%d/login", srv.C.Port))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable, got %d", resp.StatusCode)
	}
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(respBytes) != server.MaintenanceModeMsg {
		t.Fatalf("expected %q got resp %q", server.MaintenanceModeMsg, respBytes)
	}
}

func TestProdServerUsesSecureCookies(t *testing.T) {
	port, storageFolder, _, _ := setupTest(
		/*startTrackServer*/ false,
		/*useMockTrackServer*/ false, t)

	setProdConfigEnvVars(t)
	cfg := srvconfig.ProdConfig(port, storageFolder, "")
	cfg.SkipMigrations = true
	cfg.HasSecretsMasterKey = false
	srv := server.NewSrv(cfg)
	const runInMaintenanceMode = false
	go srv.Run(runInMaintenanceMode)
	for !srv.IsReady {
		time.Sleep(10 * time.Microsecond)
	}
	serverUrl := fmt.Sprintf("http://localhost:%d", srv.C.Port)

	c := &http.Client{}
	form := url.Values{}
	form.Set(routes.LogInEmailFieldName, "aang@twigg.vc")
	form.Set(routes.LogInPasswordFieldName, "yipyip")
	resp, err := c.PostForm(serverUrl+"/login", form)
	if err != nil {
		t.Fatalf("post /login failed: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatal("login failed")
	}
	cookies := resp.Cookies()
	for _, cookie := range cookies {
		if !cookie.Secure {
			t.Fatalf("got non secure cookie in response")
		}
	}
}

func TestRedirectToUpgradeIfTooManyRepos(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)

	// Register new user should have trial plan
	MockUserOAuthRegistration(srv, b, "newuser@gmail.com", "username")
	// Upgrade paying via stripe
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebHook*/ true)
	// Create three repos
	b.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "repo1",
		routes.NewRepoDescriptionParameterName: "my repo1",
	})
	b.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "repo2",
		routes.NewRepoDescriptionParameterName: "my repo2",
	})
	b.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "repo3",
		routes.NewRepoDescriptionParameterName: "my repo3",
	})
	// Mock that the subscription was canceled
	subId, stripeCustomerId := srv.StripeClientMock.GetLastStripeSubscription()
	srv.StripeClientMock.MockSubscriptionCanceled(subId, stripeCustomerId)
	makeEmptyPostRequest(srv.C.PublicUrl+routes.StripeWebhook, b.t)

	// Now the user should be redirected to the plans page
	b.Get(routes.Home)
	b.CheckCurrentPath(routes.PlansPage)
	// If they choose the trial plan they'll be redirected to the page
	// that says they must upgrade, because they have more than the allowed
	// number of repositories
	b.Post(routes.SubscribeTrialUrl, map[string]string{})
	b.CheckCurrentPath(routes.NeedUpgradePage)
	// Now they'll upgrade via stripe and it'll work out
	MockUserChoosesToPayViaStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, b, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebHook*/ true)
	b.Get(routes.Home)
	b.CheckCurrentPath(routes.Home)
}

func TestCI_IntegrationTest(t *testing.T) {
	canRunDockerJobs, _, err := runners.CheckCanRun()
	if err != nil {
		t.Fatalf("CheckCanRun: %s", err)
	}
	if !canRunDockerJobs {
		t.Skip()
		return
	}
	srv, obs := GetMockServerAndStartTrackServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commi that creates a CI file.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	jobFile := `
{
	"Name": "say-hi-on-push-or-submit",
	"Steps": [
		{
			"Run": "echo hi"
		}
	],
	"TimeoutSeconds": 10
}
`
	tw.WriteFile(cicdpublisher.CiFilename, jobFile)
	tw.Run("commit", "Create CI file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	// CI job will be pushed to the track server. Once completed, it'll send a
	// webhook informing its completion
	obs.WaitForWebhooksWithStatus(trackclient.TrackJobStatusRunning)
	obs.WaitForWebhooksWithStatus(trackclient.TrackJobStatusSuccess)

	// When submitted, the CI will also be executed
	b.Post("/aang/BookOne/c/1/lgtm", map[string]string{
		"version": "0",
	})
	obs.ClearObservedWebhooks()
	b.Post("/aang/BookOne/c/1/submit", map[string]string{
		"version": "0",
	})
	// After the submit, the CI will once again be posted to the track server.
	// After completion, a webhook will be posted
	obs.WaitForWebhooksWithStatus(trackclient.TrackJobStatusRunning)
	obs.WaitForWebhooksWithStatus(trackclient.TrackJobStatusSuccess)

	// Now we should be able to get the jobs
	b.Get("/aang/BookOne/c/1/jobs")
	var j []job.Job
	err = json.Unmarshal(b.lastResponse, &j)
	if err != nil {
		t.Fatal(err)
	}
	if len(j) != 2 {
		t.Fatalf("unexpected jobs: %v", j)
	}
	// Expect the job for the first version of the commit and the second
	if j[0].CommitVersion != 1 || j[0].Name != "say-hi-on-push-or-submit" ||
		j[0].Path != cicdpublisher.CiFilename {
		t.Fatal("unexpected job")
	}
	if j[1].CommitVersion != 0 || j[1].Name != "say-hi-on-push-or-submit" ||
		j[1].Path != cicdpublisher.CiFilename {
		t.Fatal("unexpected job")
	}

	// And the logs
	if len(obs.Jobs) != 2 {
		t.Fatalf("unexpected jobs: %d", len(obs.Jobs))
	}
	jobId := obs.Jobs[1].Id
	b.Get(fmt.Sprintf("/aang/BookOne/c/1/jobs/out?job-id=%s", jobId))
	if !strings.Contains(string(b.lastResponse), "hi\n") {
		t.Fatalf("unexpected resp: %s", b.lastResponse)
	}
}

func TestCI_MockedRunner(t *testing.T) {
	srv, obs := GetMockServerAndStartMockTrackServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commi that creates a CI file.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	jobFile := `
{
	"Name": "get-code-on-push-or-submit",
	"Steps": [
		{
			"TemplateName": "get-code"
		}
	],
	"TimeoutMinutes": 1
}
`
	tw.WriteFile("subfolder/"+cicdpublisher.CiFilename, jobFile)
	tw.Run("commit", "Create CI file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	// CI job will be pushed to the track server; expect webhooks
	obs.WaitForWebhooksWithStatus(trackclient.TrackJobStatusRunning)
	obs.WaitForWebhooksWithStatus(trackclient.TrackJobStatusSuccess)
	// Verify the webhooks
	if len(obs.Jobs) != 2 {
		t.Fatalf("unexpected jobs len: %d", len(obs.Jobs))
	}
	if obs.Jobs[0].Id != obs.Jobs[1].Id {
		t.Fatalf("inconsistend job ids")
	}
	if obs.Payloads[0].Steps[0].Run != "tw init" {
		t.Fatalf("bad step 0 Run: %s", obs.Payloads[0].Steps[0].Run)
	}
	if obs.Payloads[0].Steps[0].Dir != "." {
		t.Fatalf("bad step 0 Dir: %s", obs.Payloads[0].Steps[0].Dir)
	}
	jobId := obs.Jobs[0].Id

	// Since the job output is mocked, expect the job out to contain "mock"
	b.Get(fmt.Sprintf("/aang/BookOne/c/1/jobs/out?job-id=%s", jobId))
	if !strings.Contains(string(b.lastResponse), "mock") {
		t.Fatalf("unexpected resp: %s", b.lastResponse)
	}
}

func TestCI_BadFileFormat(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commi that creates a CI file.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	jobFile := `invalid CI file`
	tw.WriteFile(cicdpublisher.CiFilename, jobFile)
	tw.Run("commit", "Create CI file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Since the job had a bad format, we expect to see a message explaining it.
	// Lets just wait for the job to be processed
	var j []job.Job
	start := time.Now()
	for len(j) < 1 {
		b.Get("/aang/BookOne/c/1/jobs")
		err := json.Unmarshal(b.lastResponse, &j)
		if err != nil {
			t.Fatal(err)
		}
		if time.Since(start) > time.Second*20 {
			t.Fatalf("waited too long for job to run")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(j) != 1 {
		t.Fatalf("too many jobs: %v", j)
	}
	b.Get(fmt.Sprintf("/aang/BookOne/c/1/jobs/out?job-id=%s", j[0].Id()))
	if !strings.Contains(string(b.lastResponse), "invalid JSON") {
		t.Fatalf("unexpected resp: %s", b.lastResponse)
	}
}

func TestCI_TooLargeTimeout(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commi that creates a CI file.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	jobFile := `
[
    {
    "Name": "say hi in 1h",
    "Steps": [
        {
            "Run": "echo hi"
        }
    ],
    "TimeoutMilliSeconds": 3600000
    },
    {
        "Name": "say hi in 2h",
        "Steps": [
            {
                "Run": "echo hi"
            }
        ],
        "TimeoutMilliSeconds": 7200000
    },
    {
        "Name": "say hi in 3h",
        "Steps": [
            {
                "Run": "echo hi"
            }
        ],
        "TimeoutMilliSeconds": 10800000
    }
]
`
	tw.WriteFile(cicdpublisher.CiFilename, jobFile)
	tw.Run("commit", "Create CI file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Since the job had a too big combined timeout, we expect one job to be
	// created showiung that
	var j []job.Job
	start := time.Now()
	for len(j) < 1 {
		b.Get("/aang/BookOne/c/1/jobs")
		err := json.Unmarshal(b.lastResponse, &j)
		if err != nil {
			t.Fatal(err)
		}
		if time.Since(start) > time.Second*20 {
			t.Fatalf("waited too long for job to run")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(j) != 1 {
		t.Fatalf("too many jobs: %v", j)
	}
	b.Get(fmt.Sprintf("/aang/BookOne/c/1/jobs/out?job-id=%s", j[0].Id()))
	if !strings.Contains(string(b.lastResponse), "requires more execution time or resources than allowed") {
		t.Fatalf("unexpected resp: %s", b.lastResponse)
	}
}

func TestCI_ManyFiles(t *testing.T) {
	srv, obs := GetMockServerAndStartMockTrackServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commit that creates two CI files.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	ciFileStruct := []runnerlib.CiJobJson{{
		Name: "say hi",
		Steps: []runnerlib.CiJobStepJson{
			{Run: "echo hi"},
		},
		TimeoutMilliSeconds: 5000,
	}}
	ciFileBytes, _ := json.Marshal(ciFileStruct)
	tw.WriteFile(cicdpublisher.CiFilename, string(ciFileBytes))
	tw.WriteFile("subfolder1/"+cicdpublisher.CiFilename, string(ciFileBytes))
	tw.Run("commit", "Create two CI files each with one job")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	// Wait for the webhooks of each job. We expect 4 in total (2 communicating
	// the job started and 2 communicating they finished running)
	obs.WaitForNWebhooksWithStatus(trackclient.TrackJobStatusRunning, 2)
	obs.WaitForNWebhooksWithStatus(trackclient.TrackJobStatusSuccess, 2)
}

func TestCIAnalysisPending(t *testing.T) {
	// Use a huge sleep duration and disable the queue wakeup so that the queue
	// is sleeping for the whole test.
	// This ensures the commit won't be analyzed when it's pushed and the CI
	// status will remain in `prepared`
	srv, _ := GetMockServerAndStartMockTrackServer(t, WithQueueRunnerSleep(120*time.Second))
	srv.QueueRunner.DisableWakeup(t)
	start := time.Now()
	for !srv.QueueRunner.IsSleeping() {
		time.Sleep(5 * time.Millisecond)
		if time.Since(start) > 25*time.Second {
			t.Fatal("waited too long for queue to sleep")
		}
	}

	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commi that creates a CI file.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "Create CI file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	b.Get("/aang/BookOne/c/1")
	b.CheckCurrentPageContains("cistatus=\"prepared\"")
}

func TestCIAnalysisDone(t *testing.T) {
	// Use a small sleep duration for the queue so that the commit is analyzed quickly
	srv, hooks := GetMockServerAndStartMockTrackServer(t,
		WithQueueRunnerSleep(2*time.Millisecond))
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commi that creates a CI file.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	jobFile := `
{
	"Name": "say hi",
	"Steps": [
		{
			"Run": "echo hi"
		}
	],
	"TimeoutSeconds": 2000
}
`
	tw.WriteFile(cicdpublisher.CiFilename, jobFile)
	tw.Run("commit", "Create CI file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	// Wait for a webhook communicating the job started running
	hooks.WaitForWebhooksWithStatus(trackclient.TrackJobStatusRunning)
	// We expect the cistatus to be "started" and not "prepared", which
	// means the commit was analyzed and the CI jobs are running/queued
	b.Get("/aang/BookOne/c/1")
	b.CheckCurrentPageContains("cistatus=\"started\"")
}

func TestCDIntegrationTest(t *testing.T) {
	srv, hooks := GetMockServerAndStartMockTrackServer(t,
		WithQueueRunnerSleep(50*time.Millisecond))
	startAutoCiCdObs := squeue.NewPayloadTypeObserver(
		cicdqueue.QueueStartAutoCiCdRunPayloadType)
	srv.QueueRunner.AddObserver(startAutoCiCdObs)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup the tw cli and push commi that creates a CD file with 2 stages,
	// then submit it.
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	jobFileJson := runnerlib.CdJobJson{
		Name: "say hi and bye on submit or on manual",
		On:   []runnerlib.JobTrigger{runnerlib.OnSumit, runnerlib.OnManual},
		Stages: []runnerlib.CdJobStageJson{
			{
				CanAutoStart: true,
				Name:         "say hi",
				Steps: []runnerlib.CiJobStepJson{
					{Run: "echo hi"},
				},
				TimeoutMinutes: 5,
			},
			{
				CanAutoStart: true,
				Name:         "say bye",
				Steps: []runnerlib.CiJobStepJson{
					{Run: "echo bye"},
				},
				TimeoutMinutes: 5,
			},
			{
				CanAutoStart: false,
				Name:         "wait for manual trigger",
				Steps: []runnerlib.CiJobStepJson{
					{Run: "echo thanks for the manual trigger"},
				},
				TimeoutMinutes: 5,
			},
		},
	}
	jobFile, _ := json.Marshal(jobFileJson)
	tw.WriteFile(cicdpublisher.CdFilename, string(jobFile))
	tw.Run("commit", "Create CD file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	// Add LGTM and submit
	b.Post("/aang/BookOne/c/1/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/1/submit", map[string]string{
		"version": "0",
	})

	// Wait for a webhook communicating the first stage started to run
	hooks.WaitForWebhooksForPipelineStage(0, trackclient.TrackJobStatusRunning)
	// Wait for a webhook communicating the first stage succedded
	hooks.WaitForWebhooksForPipelineStage(0, trackclient.TrackJobStatusSuccess)

	// Get the job refs
	getPipelineRefs := func() []job.PipelineRef {
		b.Get("/aang/BookOne/cd-refs")
		var refs []job.PipelineRef
		err := json.Unmarshal(b.lastResponse, &refs)
		if err != nil {
			t.Fatal(err)
		}
		return refs
	}
	refs := getPipelineRefs()
	if len(refs) != 1 {
		t.Fatalf("unexpected num of refs: %d", len(refs))
	}
	if refs[0].Path != cicdpublisher.CdFilename {
		t.Fatalf("unexpected ref oath %s", refs[0].Path)
	}
	if refs[0].Name != "say hi and bye on submit or on manual" {
		t.Fatalf("unexpected ref name %s", refs[0].Name)
	}
	pipelinePath := refs[0].Path
	pipelineName := refs[0].Name

	// Get the pipeline running for that ref
	b.Get(fmt.Sprintf("/aang/BookOne/cd-refs/%s/%s", pipelinePath, pipelineName))
	var pipelines []jobshandler.FrontendPipeline
	err := json.Unmarshal(b.lastResponse, &pipelines)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("unexpected num of pipelines: %d", len(pipelines))
	}
	pipelineId := pipelines[0].Id
	if pipelines[0].NumberOfStages != 3 {
		t.Fatalf("unexpected num of stages %d", pipelines[0].NumberOfStages)
	}
	if pipelines[0].Status != job.PipelineStatusRunning {
		t.Fatalf("pipeline should be running, got %s", pipelines[0].Status)
	}
	if pipelines[0].IsCreatedByUser {
		t.Fatalf("pipeline should not be marked as created by user")
	}
	// Get the stages for that pipeline
	getStages := func() []jobshandler.FrontendPipelineStage {
		b.Get(fmt.Sprintf("/aang/BookOne/cd-refs/%s/%s/%s/stages", pipelinePath, pipelineName, pipelineId))
		var stages []jobshandler.FrontendPipelineStage
		err = json.Unmarshal(b.lastResponse, &stages)
		if err != nil {
			t.Fatal(err)
		}
		if len(stages) != 3 {
			t.Fatalf("unexpected num of stages %d", len(stages))
		}
		return stages
	}
	stages := getStages()
	for _, st := range stages {
		if st.IsResumedByUser {
			t.Fatalf("stage is marked as resumed by user")
		}
	}
	// First stage should have succeeded
	if stages[0].Status != job.JobStatusSuccess {
		t.Fatalf("pipeline stage 0 should have succedded, got %s", stages[0].Status)
	}
	// The second stage should be waiting, queued, running or already have succeeded
	if stages[1].Status != job.JobStatusWaiting &&
		stages[1].Status != job.JobStatusQueued &&
		stages[1].Status != job.JobStatusPosted &&
		stages[1].Status != job.JobStatusRunning &&
		stages[1].Status != job.JobStatusSuccess {
		t.Fatalf("pipeline stage 1 should be waiting, queued, posted, running or succeded, got %s", stages[1].Status)
	}
	// The last stage should be waiting (if stage 1 didn't finish yet)
	// or waiting-for-manual-start (if stage 1 finished)
	if stages[2].Status != job.JobStatusWaiting &&
		stages[2].Status != job.JobStatusWaitingManualStart {
		t.Fatalf("pipeline stage 2 should be waiting/waiting-manual-start, got %s", stages[2].Status)
	}

	// Wait for the stage 1 to finish; then chech that /stages returns the status
	// finished
	hooks.WaitForWebhooksForPipelineStage(1, trackclient.TrackJobStatusRunning)
	hooks.WaitForWebhooksForPipelineStage(1, trackclient.TrackJobStatusSuccess)
	stages = getStages()
	// First stage should have succeeded
	if stages[0].Status != job.JobStatusSuccess {
		t.Fatalf("pipeline stage 0 should have succedded, got %s", stages[0].Status)
	}
	// The second stage should have succeeded
	if stages[1].Status != job.JobStatusSuccess {
		t.Fatalf("pipeline stage 1 should have succedded, got %s", stages[1].Status)
	}
	// The last stage should be waiting or waiting manual start
	if stages[2].Status != job.JobStatusWaiting &&
		stages[2].Status != job.JobStatusWaitingManualStart {
		t.Fatalf("pipeline stage 2 should be waiting-manual-start, got %s", stages[2].Status)
	}

	// Eventually, stage2 should have the waiting-manual-start status
	stage2IsWaitingManualStart := stages[2].Status == job.JobStatusWaitingManualStart
	start := time.Now()
	for !stage2IsWaitingManualStart {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 30*time.Second {
			t.Fatalf("waited too long for stage 2 to become waiting-manual-start")
		}
		stages = getStages()
		stage2IsWaitingManualStart = stages[2].Status == job.JobStatusWaitingManualStart
	}

	// Manually resume the stage2
	b.Post(fmt.Sprintf("/aang/BookOne/cd-refs/%s/%s/%s/stages/2/manual-resume",
		pipelinePath, pipelineName, pipelineId), map[string]string{})
	stages = getStages()
	if !stages[2].IsResumedByUser {
		t.Fatalf("stage2 not marked as resumed by user")
	}
	if stages[2].ResumedByUsername != "aang" {
		t.Fatalf("stage2 ResumedByUsername=%s, expected aang", stages[2].ResumedByUsername)
	}
	hooks.WaitForWebhooksForPipelineStage(2, trackclient.TrackJobStatusRunning)
	hooks.WaitForWebhooksForPipelineStage(2, trackclient.TrackJobStatusSuccess)

	// Get the output of each stage. Since this test uses a mocked runner,
	// we expect all of them to have "mocked out"
	checkStageOutput := func(stage int) {
		b.Get(fmt.Sprintf("/aang/BookOne/cd-refs/%s/%s/%s/stages/%d/out",
			pipelinePath, pipelineName, pipelineId, stage))
		if string(b.lastResponse) != "mocked out" {
			t.Fatalf("unexpected resp for pipeline stage=%d out: %s", stage, b.lastResponse)
		}
	}
	checkStageOutput(0)
	checkStageOutput(1)
	checkStageOutput(2)

	// Manually launch a run at c1v1
	b.Post(fmt.Sprintf("/aang/BookOne/cd-manual-launch/%s/%s/1/1", pipelinePath, pipelineName),
		map[string]string{})
	hooks.WaitForWebhooksForPipelineStage(0, trackclient.TrackJobStatusSuccess)
	hooks.WaitForWebhooksForPipelineStage(1, trackclient.TrackJobStatusSuccess)
	// Check the pipelines endpoint
	b.Get(fmt.Sprintf("/aang/BookOne/cd-refs/%s/%s", pipelinePath, pipelineName))
	err = json.Unmarshal(b.lastResponse, &pipelines)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("unexpected num of pipelines: %d", len(pipelines))
	}
	if !pipelines[0].IsCreatedByUser {
		t.Fatalf("latest pipeline not marked as created by user")
	}
	if pipelines[0].CreatedByUsername != "aang" {
		t.Fatalf("unexpected username of pipeline: %s", pipelines[0].CreatedByUsername)
	}

	// Create a commit that changes the CD file so that its a new name
	startAutoCiCdObs.Reset()
	newJobFileJson := jobFileJson
	newJobFileJson.Name = jobFileJson.Name + "_MODIFIED"
	newJobFile, _ := json.Marshal(newJobFileJson)
	tw.WriteFile(cicdpublisher.CdFilename, string(newJobFile))
	tw.Run("commit", "Modify CD file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	// Wait for the CiCd run to finish
	startAutoCiCdObs.WaitForOkCount(1, t)
	// The job ref should still exist before submitting
	refs = getPipelineRefs()
	if len(refs) != 1 {
		t.Fatalf("unexpected num of refs: %d", len(refs))
	}
	if refs[0].Name != jobFileJson.Name {
		t.Fatalf("unexpected name %s", refs[0].Name)
	}
	// Add LGTM and submit. After submitting, the old ref should be
	// deleted and the new one should appear.
	startAutoCiCdObs.Reset()
	b.Post("/aang/BookOne/c/2/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/2/submit", map[string]string{
		"version": "0",
	})
	startAutoCiCdObs.WaitForOkCount(1, t)
	refs = getPipelineRefs()
	if len(refs) != 1 {
		t.Fatalf("unexpected num of refs: %d", len(refs))
	}
	if refs[0].Name != newJobFileJson.Name {
		t.Fatalf("unexpected name %s", refs[0].Name)
	}

	// Create a commit that deletes the CD file
	startAutoCiCdObs.Reset()
	tw.DeleteFile(cicdpublisher.CdFilename)
	tw.Run("commit", "Delete CD file")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")
	// Wait for the CiCd run to finish
	startAutoCiCdObs.WaitForOkCount(1, t)
	// The job ref should still exist before submitting
	refs = getPipelineRefs()
	if len(refs) != 1 {
		t.Fatalf("unexpected num of refs: %d", len(refs))
	}
	// Add LGTM and submit
	startAutoCiCdObs.Reset()
	b.Post("/aang/BookOne/c/3/lgtm", map[string]string{
		"version": "0",
	})
	b.Post("/aang/BookOne/c/3/submit", map[string]string{
		"version": "0",
	})
	// Wait for the CiCd run to finish
	startAutoCiCdObs.WaitForOkCount(1, t)
	refs = getPipelineRefs()
	if len(refs) != 0 {
		t.Fatalf("unexpected num of refs: %d", len(refs))
	}
}

func TestAddLgtmWithoutOwnerThenWithOwner(t *testing.T) {
	srv := GetMockServer(t)

	// Momo registers, pays for a plan and creates a repo
	momoB := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, momoB, "momo@northern-temple.air", "momo")
	MockUserChoosesToPayViaStripe(srv, momoB, srv.StripeClientMock.GetLatestSoloPriceId(), 1)
	MockUserFinishedPayingInStripe(srv, momoB, srv.StripeClientMock.GetLatestSoloPriceId(), 1,
		/*postWebhookEvent*/ true)
	momoB.Post(routes.NewRepoUrl, map[string]string{
		routes.NewRepoNameParameterName:        "momorepo",
		routes.NewRepoDescriptionParameterName: "my repo description",
	})
	// Appa register its account
	appaB := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthRegistration(srv, appaB, "appa@northern-temple.air", "appa")
	appaB.Get("/home")
	appaB.CheckCurrentPath("/home")
	// Momo grants appa access to the repo
	MockUserGrantsPermission(srv, momoB, "momo", "momorepo", "appa")

	// Momo and appa create a cli key
	momoB.Post(routes.GenerateCLIKey, nil)
	momoCliKey := srv.KeysMock.GetLastRandomCliKey()
	appaB.Post(routes.GenerateCLIKey, nil)
	appaCliKey := srv.KeysMock.GetLastRandomCliKey()

	// Momo and appa create a client and configure the server
	momoTw := cli.NewTestHelper(t)
	momoTw.SetServerRootUrl(srv.C.PublicUrl)
	momoTw.Run("init")
	momoTw.Run("server", "momo/momorepo")
	momoTw.Run("key", momoCliKey)
	appaTw := cli.NewTestHelper2(t)
	appaTw.SetServerRootUrl(srv.C.PublicUrl)
	appaTw.Run("init")
	appaTw.Run("server", "momo/momorepo")
	appaTw.Run("key", appaCliKey)

	// Momo pushes a commit that creates an OWNERS file
	momoTw.WriteFile("OWNERS", "momo")
	momoTw.Run("commit", "create OWNERS")
	momoTw.Run("push")
	momoTw.CheckOutContains("push succeeded")
	// Appa add LGTM and submits it
	appaB.Post("/momo/momorepo/c/1/lgtm", map[string]string{
		"version": "0",
	})
	appaB.Post("/momo/momorepo/c/1/submit", map[string]string{
		"version": "0",
	})

	// Appa pushes a random commit
	appaTw.WriteFile("a.txt", "aaa")
	appaTw.Run("commit", "create a.txt")
	appaTw.Run("push")
	appaTw.CheckOutContains("push succeeded")

	// Appa ads LGTM and tries to submit but fails,
	// due to missing owners
	appaB.Post("/momo/momorepo/c/2/lgtm", map[string]string{
		"version": "0",
	})
	appaB.CheckPostErrors("/momo/momorepo/c/2/submit", map[string]string{
		"version": "0",
	})

	// Momo LGTMs the commit
	momoB.Post("/momo/momorepo/c/2/lgtm", map[string]string{
		"version": "0",
	})

	// The commit can now be submitted by anyone
	appaB.Post("/momo/momorepo/c/2/submit", map[string]string{
		"version": "0",
	})

	// Momo pushes a WIP commit
	momoTw.WriteFile("b.txt", "bbb")
	momoTw.Run("commit", "Wip b")
	momoTw.Run("push")
	momoTw.CheckOutContains("push succeeded")

	// Momo LGTMs the commit
	momoB.Post("/momo/momorepo/c/3/lgtm", map[string]string{
		"version": "0",
	})

	// Appa tries to submit but fails,
	// due to WIP commit
	appaB.CheckPostErrors("/momo/momorepo/c/2/submit", map[string]string{
		"version": "0",
	})

	// Momo remove WIP in the commit
	momoTw.Run("amend", "new b")
	momoTw.Run("push")
	momoTw.CheckOutContains("push succeeded")

	// The commit can now be submitted by anyone
	appaB.Post("/momo/momorepo/c/3/submit", map[string]string{
		"version": "1",
	})

	appaB.Get("/momo/momorepo")
}

func TestTwiggPullWithTwiggTokenWithBadRepoId(t *testing.T) {
	srv := GetMockServer(t)
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")

	// Create a token that is bound to some other repoId; not the test one.
	// This token should now allow pulling from the test repo.
	const badRepoId uint64 = 9999
	actions := []twiggtoken.TokenAction{twiggtoken.TokenActionPull}
	actionsArg := []string{""}
	twToken, err := twiggtoken.NewTwiggToken(
		badRepoId,
		/*commit*/ 99,
		/*commitVersion*/ 87,
		actions,
		actionsArg,
		5*time.Minute,
		sign.NewSigner(srv.C.TwiggTokenSigningKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	tw.Run("key", twToken)
	tw.Run("pull")
	tw.CheckOutContains("permission denied")
}

func TestTwiggPullWithTwiggToken(t *testing.T) {
	srv := GetMockServer(t)
	// Setup the tw cli
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")

	duration := 5 * time.Minute
	const repoId uint64 = 1 // This must match the repoId that is being pulled
	var commitServerId uint64 = 99
	var commitVersion uint64 = 42
	actions := []twiggtoken.TokenAction{twiggtoken.TokenActionPull}
	actionsArg := []string{""}

	twToken, err := twiggtoken.NewTwiggToken(
		repoId,
		commitServerId,
		commitVersion,
		actions,
		actionsArg,
		duration,
		sign.NewSigner(srv.C.TwiggTokenSigningKey),
	)
	if err != nil {
		t.Fatal(err)
	}

	tw.Run("key", twToken)

	tw.Run("pull")
	tw.CheckOutContains("up to date")

	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("permission denied")
}
func TestAddReviewers(t *testing.T) {

	srv := GetMockServer(t)

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")
	bAang.Get(routes.UserSettings)
	bAang.Post(routes.GenerateCLIKey, nil)

	// Create commit
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	type addReviewersBody struct {
		Usernames []string
	}

	// Add valid reviewers
	status := bAang.PostJson("/aang/BookOne/c/1/reviewers", addReviewersBody{Usernames: []string{"katara", "sokka"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bAang.lastResponse)
	}

	// Self add as reviewer is allowed
	status = bAang.PostJson("/aang/BookOne/c/1/reviewers", addReviewersBody{Usernames: []string{"aang"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200 for self add, got %d body=%s", status, bAang.lastResponse)
	}

	// Invalid username
	status = bAang.PostJson("/aang/BookOne/c/1/reviewers", addReviewersBody{Usernames: []string{"does-not-exist"}})
	if status == http.StatusOK {
		t.Fatal("expected error for invalid username")
	}

	// Empty list
	status = bAang.PostJson("/aang/BookOne/c/1/reviewers", addReviewersBody{Usernames: []string{}})
	if status == http.StatusOK {
		t.Fatal("expected error for empty list")
	}

	// Too many usernames
	status = bAang.PostJson("/aang/BookOne/c/1/reviewers", addReviewersBody{
		Usernames: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u"},
	})
	if status == http.StatusOK {
		t.Fatal("expected error for too many usernames")
	}

	// Notifications test for AddReviewers
	// aang added katara and sokka as reviewers
	// -> katara and sokka should each receive "aang added you as a reviewer of c/1"
	// -> aang should NOT receive notifications (is the commit author)
	bKatara := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bKatara, "katara@twigg.vc")
	bKatara.Get(routes.Notifications)
	var kataraNotifs notifications.GetNotificationsResponse
	if err := json.Unmarshal(bKatara.lastResponse, &kataraNotifs); err != nil {
		t.Fatalf("failed to unmarshal katara notifications: %v", err)
	}

	if len(kataraNotifs.Notifications) != 1 {
		t.Fatalf("expected 1 notification for katara, got %d", len(kataraNotifs.Notifications))
	}
	if !strings.Contains(kataraNotifs.Notifications[0].Message, "aang added you as a reviewer of c/1") {
		t.Fatalf("unexpected message: %q", kataraNotifs.Notifications[0].Message)
	}

	bSokka := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bSokka, "sokka@twigg.vc")
	bSokka.Get(routes.Notifications)
	var sokkaNotifs notifications.GetNotificationsResponse
	err := json.Unmarshal(bSokka.lastResponse, &sokkaNotifs)
	if err != nil {
		t.Fatalf("failed to unmarshal sokka notifications: %v", err)
	}

	if len(sokkaNotifs.Notifications) != 1 {
		t.Fatalf("expected 1 notification for sokka, got %d", len(sokkaNotifs.Notifications))
	}
	if !strings.Contains(sokkaNotifs.Notifications[0].Message, "aang added you as a reviewer of c/1") {
		t.Fatalf("unexpected message: %q", sokkaNotifs.Notifications[0].Message)
	}

	bAang.Get(routes.Notifications)
	var aangNotifs notifications.GetNotificationsResponse
	if err := json.Unmarshal(bAang.lastResponse, &aangNotifs); err != nil {
		t.Fatalf("failed to unmarshal aang notifications: %v", err)
	}
	if len(aangNotifs.Notifications) != 0 {
		t.Fatalf("expected 0 notifications for aang (is commit author), got %d", len(aangNotifs.Notifications))
	}

	// katara (not the commit author) adds toph as reviewer
	// -> toph receives "katara added you as a reviewer of c/1"
	// -> aang (commit author) receives "katara added toph as a reviewer of c/1"
	status = bKatara.PostJson("/aang/BookOne/c/1/reviewers", addReviewersBody{Usernames: []string{"toph"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bKatara.lastResponse)
	}

	bToph := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bToph, "toph@twigg.vc")
	bToph.Get(routes.Notifications)
	var tophNotifs notifications.GetNotificationsResponse
	err = json.Unmarshal(bToph.lastResponse, &tophNotifs)
	if err != nil {
		t.Fatalf("failed to unmarshal toph notifications: %v", err)
	}

	if len(tophNotifs.Notifications) != 1 {
		t.Fatalf("expected 1 notification for toph, got %d", len(tophNotifs.Notifications))
	}
	if !strings.Contains(tophNotifs.Notifications[0].Message, "katara added you as a reviewer of c/1") {
		t.Fatalf("unexpected message: %q", tophNotifs.Notifications[0].Message)
	}

	bAang.Get(routes.Notifications)
	err = json.Unmarshal(bAang.lastResponse, &aangNotifs)
	if err != nil {
		t.Fatalf("failed to unmarshal aang notifications: %v", err)
	}
	if len(aangNotifs.Notifications) != 1 {
		t.Fatalf("expected 1 notification for aang, got %d", len(aangNotifs.Notifications))
	}
	if !strings.Contains(aangNotifs.Notifications[0].Message, "katara added toph as a reviewer of c/1") {
		t.Fatalf("unexpected message: %q", aangNotifs.Notifications[0].Message)
	}
}
func TestRemoveReviewers(t *testing.T) {
	srv := GetMockServer(t)

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")
	bAang.Get(routes.UserSettings)
	bAang.Post(routes.GenerateCLIKey, nil)

	// Create commit
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	type reviewersBody struct {
		Usernames []string
	}

	// Add katara and sokka as reviewers first
	status := bAang.PostJson("/aang/BookOne/c/1/reviewers", reviewersBody{Usernames: []string{"katara", "sokka"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bAang.lastResponse)
	}

	// Remove valid reviewer
	status = bAang.PostJson("/aang/BookOne/c/1/rm-reviewers", reviewersBody{Usernames: []string{"katara"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bAang.lastResponse)
	}

	// Invalid username
	status = bAang.PostJson("/aang/BookOne/c/1/rm-reviewers", reviewersBody{Usernames: []string{"does-not-exist"}})
	if status == http.StatusOK {
		t.Fatal("expected error for invalid username")
	}

	// Empty list
	status = bAang.PostJson("/aang/BookOne/c/1/rm-reviewers", reviewersBody{Usernames: []string{}})
	if status == http.StatusOK {
		t.Fatal("expected error for empty list")
	}

	if status == http.StatusOK {
		t.Fatal("expected error for too many usernames")
	}

	// aang (commit author) removes sokka as reviewer
	// -> aang should NOT receive a notification (no self-notify)
	status = bAang.PostJson("/aang/BookOne/c/1/rm-reviewers", reviewersBody{Usernames: []string{"sokka"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bAang.lastResponse)
	}

	bAang.Get(routes.Notifications)
	var aangNotifs notifications.GetNotificationsResponse
	err := json.Unmarshal(bAang.lastResponse, &aangNotifs)
	if err != nil {
		t.Fatalf("failed to unmarshal aang notifications: %v", err)
	}
	if len(aangNotifs.Notifications) != 0 {
		t.Fatalf("expected 0 notifications for aang (is commit author), got %d", len(aangNotifs.Notifications))
	}

	// re-add sokka via katara (not the commit author)
	// -> aang receives "katara added sokka as a reviewer of c/1"
	bKatara := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bKatara, "katara@twigg.vc")
	status = bKatara.PostJson("/aang/BookOne/c/1/reviewers", reviewersBody{Usernames: []string{"sokka"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200 re-adding sokka, got %d body=%s", status, bKatara.lastResponse)
	}

	bAang.Get(routes.Notifications)
	err = json.Unmarshal(bAang.lastResponse, &aangNotifs)
	if err != nil {
		t.Fatalf("failed to unmarshal aang notifications after katara re-added sokka: %v", err)
	}
	if len(aangNotifs.Notifications) != 1 {
		t.Fatalf("expected 1 notification for aang after katara re-added sokka, got %d", len(aangNotifs.Notifications))
	}
	if !strings.Contains(aangNotifs.Notifications[0].Message, "katara added sokka as a reviewer of c/1") {
		t.Fatalf("unexpected message: %q", aangNotifs.Notifications[0].Message)
	}

	// katara (not the commit author) removes sokka as reviewer
	// -> aang (commit author) receives "katara removed sokka as a reviewer of c/1"
	status = bKatara.PostJson("/aang/BookOne/c/1/rm-reviewers", reviewersBody{Usernames: []string{"sokka"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bKatara.lastResponse)
	}

	bAang.Get(routes.Notifications)
	err = json.Unmarshal(bAang.lastResponse, &aangNotifs)
	if err != nil {
		t.Fatalf("failed to unmarshal aang notifications after remove: %v", err)
	}
	if len(aangNotifs.Notifications) != 2 {
		t.Fatalf("expected 2 notifications for aang, got %d", len(aangNotifs.Notifications))
	}
	if !strings.Contains(aangNotifs.Notifications[0].Message, "katara removed sokka as a reviewer of c/1") {
		t.Fatalf("unexpected message: %q", aangNotifs.Notifications[0].Message)
	}
}

func TestTwiggWebClient_GetSecretValsFromTwiggWeb(t *testing.T) {
	// WARNING: This test assumes /aang/BookOne/ id = 1

	// Start server and add secrets to repo
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Post("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  "aaa",
		routes.RepoSecretValueParamName: "value-a",
	})
	b.Post("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  "bbb",
		routes.RepoSecretValueParamName: "value-b",
	})
	b.Post("/aang/BookOne/settings/repo-secret", map[string]string{
		routes.RepoSecretNameParamName:  "NOT-ALLOWED-SECRET",
		routes.RepoSecretValueParamName: "random-value",
	})

	// Happy path:
	twToken, err := twiggtoken.NewTwiggToken(
		/*repoId, commitServerId, commitVersion*/ 1, 1, 1,
		[]twiggtoken.TokenAction{
			twiggtoken.TokenActionGetSecret,
			twiggtoken.TokenActionGetSecret,
		},
		[]string{"aaa", "bbb"},
		time.Hour,
		sign.NewSigner(srv.C.TwiggTokenSigningKey),
	)
	if err != nil {
		t.Fatal(err)
	}
	c := twiggwebclient.NewClient(b.serverUrl, srv.C.TwiggServerKey)
	requiredSecrets, isNotFoundOrForbiddenErr, err := c.GetSecretValsFromTwiggWeb([]string{"aaa", "bbb"}, twToken)
	if err != nil {
		t.Fatal(err)
	}
	if isNotFoundOrForbiddenErr {
		t.Fatal("ecpected isNotFoundOrForbiddenErr to be false, since err != nil")
	}
	if len(requiredSecrets) != 2 {
		t.Fatalf("unexpected len of requiredSecrets. got: %d", len(requiredSecrets))
	}
	if requiredSecrets["aaa"] != "value-a" {
		t.Fatalf("unexpected 'aaa' value. got: %q", requiredSecrets["aaa"])
	}
	if requiredSecrets["bbb"] != "value-b" {
		t.Fatalf("unexpected 'bbb' value. got: %q", requiredSecrets["bbb"])
	}

	// Not found:
	twToken, err = twiggtoken.NewTwiggToken(
		/*repoId, commitServerId, commitVersion*/ 1, 1, 1,
		[]twiggtoken.TokenAction{
			twiggtoken.TokenActionGetSecret,
			twiggtoken.TokenActionGetSecret,
			twiggtoken.TokenActionGetSecret,
		},
		[]string{"aaa", "bbb", "NOT-FOUND-SECRET"},
		time.Hour,
		sign.NewSigner(srv.C.TwiggTokenSigningKey),
	)
	if err != nil {
		t.Fatal(err)
	}
	requiredSecrets, isNotFoundOrForbiddenErr, err = c.GetSecretValsFromTwiggWeb(
		[]string{
			"aaa",
			"bbb",
			"NOT-FOUND-SECRET",
		},
		twToken,
	)
	if !isNotFoundOrForbiddenErr {
		t.Fatal("got isNotFoundOrForbiddenErr==false, expected not found error")
	}
	if err == nil {
		t.Fatal("got nil err for not-found-secret secret")
	}

	// Not allowed secret
	requiredSecrets, isNotFoundOrForbiddenErr, err = c.GetSecretValsFromTwiggWeb(
		[]string{
			"aaa",
			"bbb",
			"NOT-ALLOWED-SECRET",
		},
		twToken,
	)
	if !isNotFoundOrForbiddenErr {
		t.Fatal("got isForbiddenErr==false, expected it ot be true")
	}
	if err == nil {
		t.Fatal("got nil err for not allowed secret")
	}
}
func TestGetReviewers(t *testing.T) {
	srv := GetMockServer(t)

	bAang := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, bAang, "aang@twigg.vc")
	bAang.Get(routes.UserSettings)
	bAang.Post(routes.GenerateCLIKey, nil)

	// Create commit
	tw := cli.NewTestHelper(t)
	tw.SetServerRootUrl(srv.C.PublicUrl)
	tw.Run("init")
	tw.Run("server", "aang/BookOne")
	tw.Run("key", srv.KeysMock.GetLastRandomCliKey())
	tw.WriteFile("a.txt", "aaa")
	tw.Run("commit", "create a.txt")
	tw.Run("push")
	tw.CheckOutContains("push succeeded")

	type reviewersBody struct {
		Usernames []string
	}

	// No reviewers initially
	bAang.Get("/aang/BookOne/c/1/get-reviewers")
	var reviewers []string
	if err := json.Unmarshal(bAang.lastResponse, &reviewers); err != nil {
		t.Fatalf("failed to unmarshal reviewers: %v", err)
	}
	if len(reviewers) != 0 {
		t.Fatalf("expected 0 reviewers, got %d", len(reviewers))
	}

	// Add katara and sokka as reviewers
	status := bAang.PostJson("/aang/BookOne/c/1/reviewers", reviewersBody{Usernames: []string{"katara", "sokka"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bAang.lastResponse)
	}

	// GET should return both
	bAang.Get("/aang/BookOne/c/1/get-reviewers")
	if err := json.Unmarshal(bAang.lastResponse, &reviewers); err != nil {
		t.Fatalf("failed to unmarshal reviewers: %v", err)
	}
	if len(reviewers) != 2 {
		t.Fatalf("expected 2 reviewers, got %d", len(reviewers))
	}
	if !slices.Contains(reviewers, "katara") {
		t.Fatalf("expected katara in reviewers, got %v", reviewers)
	}
	if !slices.Contains(reviewers, "sokka") {
		t.Fatalf("expected sokka in reviewers, got %v", reviewers)
	}

	// Remove katara
	status = bAang.PostJson("/aang/BookOne/c/1/rm-reviewers", reviewersBody{Usernames: []string{"katara"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", status, bAang.lastResponse)
	}

	// GET should return only sokka
	bAang.Get("/aang/BookOne/c/1/get-reviewers")
	if err := json.Unmarshal(bAang.lastResponse, &reviewers); err != nil {
		t.Fatalf("failed to unmarshal reviewers: %v", err)
	}
	if len(reviewers) != 1 {
		t.Fatalf("expected 1 reviewer, got %d", len(reviewers))
	}
	if reviewers[0] != "sokka" {
		t.Fatalf("expected sokka, got %s", reviewers[0])
	}
}
func TestGetCanSubmitCommits(t *testing.T) {
	srv := GetMockServer(t)
	b := NewTestBrowser(srv.C.PublicUrl, t)
	MockUserOAuthSignIn(srv, b, "aang@twigg.vc")
	b.Get(routes.UserSettings)
	b.CheckCurrentPath(routes.UserSettings)
	b.Post(routes.GenerateCLIKey, nil)

	// Setup two CLI clients simulating two devs working in parallel
	tw1 := cli.NewTestHelper(t)
	tw1.SetServerRootUrl(srv.C.PublicUrl)
	tw1.Run("init")
	tw1.Run("server", "aang/BookOne")
	tw1.Run("key", srv.KeysMock.GetLastRandomCliKey())

	tw2 := cli.NewTestHelper2(t)
	tw2.SetServerRootUrl(srv.C.PublicUrl)
	tw2.Run("init")
	tw2.Run("server", "aang/BookOne")
	tw2.Run("key", srv.KeysMock.GetLastRandomCliKey())

	// Both start from the same base - tw1 and tw2 both modify a.txt
	tw1.WriteFile("a.txt", "version from tw1")
	tw1.Run("commit", "tw1 edits a.txt")
	tw1.Run("push")
	tw1.CheckOutContains("push succeeded")

	tw2.WriteFile("a.txt", "version from tw2")
	tw2.Run("commit", "tw2 edits a.txt")
	tw2.Run("push")
	tw2.CheckOutContains("push succeeded")

	// Both commits should be submittable before either is submitted
	b.Get("/aang/BookOne/can-submit-commits?c=1&c=2")
	var resp commit.HandleGetCanSubmitCommitsResponse
	if err := json.Unmarshal(b.lastResponse, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp["1"].CanSubmit {
		t.Fatalf("expected commit 1 to be submittable, got reason=%q", resp["1"].CantSubmitReason)
	}
	if !resp["2"].CanSubmit {
		t.Fatalf("expected commit 2 to be submittable before conflict, got reason=%q", resp["2"].CantSubmitReason)
	}

	// Submit commit 1 first
	b.Post("/aang/BookOne/c/1/lgtm", map[string]string{"version": "0"})
	b.Post("/aang/BookOne/c/1/submit", map[string]string{"version": "0"})

	// Now commit 2 should not be submittable because submitting would cause a rebase conflict
	b.Get("/aang/BookOne/can-submit-commits?c=2")
	if err := json.Unmarshal(b.lastResponse, &resp); err != nil {
		t.Fatal(err)
	}
	if resp["2"].CanSubmit {
		t.Fatal("expected commit 2 to not be submittable due to rebase conflict")
	}
	if resp["2"].CantSubmitReason != "would-cause-rebase-conflict" {
		t.Fatalf("expected reason 'would-cause-rebase-conflict', got %q", resp["2"].CantSubmitReason)
	}
}