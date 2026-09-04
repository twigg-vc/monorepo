package client

import (
	"fmt"
	"io"
	"monorepo/twigg/xchange"
	"net/http"
	"strings"
)

func (a tw) SetNextServerId(serverUrl string, apiKey string, id uint64) (
	notOkMsg string, err error) {

	url := fmt.Sprintf("%s%s?%s=%d",
		serverUrl, SetServerIdEndpoint, SetServerIdQueryParam, id)
	req, err := http.NewRequest(SetServerIdMethod, url, nil)
	if err != nil {
		return "", err
	}
	xchange.SetTwiggHeaderInRequest(req)
	xchange.SetApiKeyHeader(apiKey, req)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", ErrFailedToReachServer
	}
	defer resp.Body.Close()
	if !xchange.MightBeTwiggResponse(resp) {
		return "", ErrNotTwiggServer
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return strings.TrimSpace(string(body)), nil
	}
	return "", nil
}
