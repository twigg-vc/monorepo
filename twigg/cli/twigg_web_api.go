package cli

import twiggwebclient "monorepo/twigg-web/twigg-web-client"

// Simple helper that logs an error and returns false if the server and api
// were not setup yet.
func (a *app) checkServerIsSetup() (ok bool) {
	if a.s.ServerUrl == "" {
		a.logError(serverUrlNotSet)
		return
	}
	if a.s.ApiKey == "" {
		a.logError(apiKeyNotSet)
		return
	}
	return true
}

func (a *app) pingTwiggWeb(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}

	client := twiggwebclient.NewCliKeyClient(a.s.ServerUrl, a.s.ApiKey)
	ok, err := client.Ping()
	if err != nil || !ok {
		a.logError(err.Error())
		return
	}
	a.logSuccess("ok")
}
