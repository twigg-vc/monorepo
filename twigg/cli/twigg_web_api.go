package cli

import twiggwebclient "monorepo/twigg-web/twigg-web-client"

func (a *app) pingTwiggWeb(args commandArgs) {
	if a.s.ServerUrl == "" {
		a.logError(serverUrlNotSet)
		return
	}
	if a.s.ApiKey == "" {
		a.logError(apiKeyNotSet)
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
