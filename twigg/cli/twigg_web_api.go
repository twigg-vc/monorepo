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

func (a *app) getTwiggWebClient() twiggwebclient.CliKeyClient {
	return twiggwebclient.NewCliKeyClient(a.s.ServerUrl, a.s.ApiKey)
}

func (a *app) pingTwiggWeb(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}

	client := a.getTwiggWebClient()
	ok, err := client.Ping()
	if err != nil || !ok {
		a.logError(err.Error())
		return
	}
	a.logSuccess("ok")
}

func (a *app) getBug(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}
	a.logSuccess("wip")
}
