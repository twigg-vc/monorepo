package twiggwebclient

import (
	"net/http"
)

// MUST BE INITIALIZED WITH `NewClient`
// Client for services to communicate with the twigg web server
type Client struct {
	c client
}

// Instantiates a new Client ready to be used
func NewClient(twiggServerUrl string, twiggServerKey string) Client {
	return Client{
		c: client{
			httpClient:     &http.Client{},
			twiggServerUrl: twiggServerUrl,
			twiggServerKey: twiggServerKey,
		},
	}
}

func (c Client) GetSecretValsFromTwiggWeb(
	requiredSecretsNames []string,
	twiggToken string,
) (requiredSecrets map[string]string, isNotFoundOrForbiddenErr bool, err error) {
	return c.c.getSecretValsFromTwiggWeb(requiredSecretsNames, twiggToken)
}

// MUST BE INITIALIZED WITH `NewClient`
// Client for services to communicate with the twigg web server via CLI key
type CliKeyClient struct {
	c cliclient
}

// Instantiates a new Client ready to be used
func NewCliKeyClient(repoUrl string, twiggCliKey string) CliKeyClient {
	return CliKeyClient{
		c: cliclient{
			httpClient:  &http.Client{},
			repoUrl:     repoUrl,
			twiggCliKey: twiggCliKey,
		},
	}
}

func (c CliKeyClient) Ping() (ok bool, err error) {
	return c.c.ping()
}
