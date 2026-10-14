package twiggwebclient

import (
	"errors"
	"fmt"
	"monorepo/twigg-web/routes"
	"monorepo/twigg/xchange"
	"net/http"
)

type cliclient struct {
	httpClient  *http.Client
	repoUrl     string
	twiggCliKey string
}

var (
	ErrFailedToReachServer = errors.New("failed to reach the server")
)

func (c cliclient) ping() (ok bool, err error) {
	req, err := http.NewRequest("GET", c.repoUrl+"/ping", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create GET request to pull: %s", err))
	}
	xchange.SetApiKeyHeader(c.twiggCliKey, req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("%w: %s", ErrFailedToReachServer, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("%w: %s", ErrFailedToReachServer, resp.Status)
	}
	return true, nil
}

var _ int = func() int {
	if routes.CliClientPing != "/{owner}/{repo}/ping" {
		panic("CliClientPing pattern changed! did you update the client?")
	}
	return 0
}()
