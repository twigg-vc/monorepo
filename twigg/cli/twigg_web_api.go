package cli

import (
	"encoding/json"
	"fmt"
	twiggwebclient "monorepo/twigg-web/twigg-web-client"
)

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

func (a *app) setCommitDescription(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}
	var serverId uint64
	if args.commit0 == "" {
		if !a.s.Current.HasServerL {
			a.logError(cantDescribeNotPushed)
			return
		}
		serverId = uint64(a.s.Current.ServerL)
	} else {
		match, id, hasVersion, _ := a.tryParseServerCommitString(args.commit0)
		if !match {
			a.logError(badCommitServerSyntax)
			return
		}
		if hasVersion {
			a.logError(cantDescribeCommitVersion)
			return
		}
		serverId = id
	}
	client := a.getTwiggWebClient()
	isNotFoundErr, err := client.SetCommitDescription(serverId, args.description)
	if isNotFoundErr {
		a.logError(fmt.Sprintf("c/%d not found", serverId))
		return
	}
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logSuccess("ok")
}

func (a *app) getBug(args commandArgs) {
	if !a.checkServerIsSetup() {
		return
	}
	client := a.getTwiggWebClient()
	bug, isNotFoundErr, err := client.GetBugAndEvents(args.bugNumber)
	if isNotFoundErr {
		a.logError(fmt.Sprintf("b/%d not found", args.bugNumber))
		return
	}
	if err != nil {
		a.logError(err.Error())
		return
	}
	b, err := json.MarshalIndent(bug, "", "  ") // prefix, indent
	if err != nil {
		a.logError(err.Error())
		return
	}
	a.logInfo(string(b))
}
