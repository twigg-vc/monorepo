package twiggwebclient

import (
	"encoding/json"
	"fmt"
	"monorepo/twigg-web/routes"
	"monorepo/twigg-web/services/twiggtoken"
	"net/http"
	"net/url"
)

type client struct {
	httpClient     *http.Client
	twiggServerUrl string
	twiggServerKey string
}

func (c client) getSecretValsFromTwiggWeb(
	requiredSecretsNames []string,
	twiggToken string,
) (map[string]string, bool, error) {

	params := url.Values{}
	for _, s := range requiredSecretsNames {
		params.Add(routes.RepoSecretNameParamName, s)
	}

	fullURL := fmt.Sprintf("%s%s?%s", c.twiggServerUrl, routes.TrackWebhooksSecrets, params.Encode())
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, false, err
	}

	twiggtoken.SetTwiggTokenInHeader(twiggToken, req)
	req.Header.Set("TwiggServerKey", c.twiggServerKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		requiredSecrets := map[string]string{}
		err = json.NewDecoder(resp.Body).Decode(&requiredSecrets)
		return requiredSecrets, false, err
	case http.StatusForbidden:
		return nil, true, fmt.Errorf("get secrets got a forbidden status. token: %q", twiggToken)
	case http.StatusNotFound:
		return nil, true, fmt.Errorf("get secrets got a not-found status. token: %q", twiggToken)
	default:
		return nil, false, fmt.Errorf("get secrets failed with status %d", resp.StatusCode)
	}
}
