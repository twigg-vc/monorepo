package trackclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/wrappers"
	"net/http"
	"strings"
)

type client struct {
	trackServerUrl string
	trackKey       string
	httpClient     *http.Client
}

func (c client) HealthIsOk() (bool, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/health", c.trackServerUrl), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create get /health request: %s", err)
	}
	setAuthHeader(req, c.trackKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("get /health failed: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, fmt.Errorf("get /health status: %s", resp.Status)
}
func (c client) Cancel(jobId string) error {
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/c-job/%s", c.trackServerUrl, jobId),
		nil)
	if err != nil {
		return fmt.Errorf("new put job request: %s", err)
	}
	setAuthHeader(req, c.trackKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("put job: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1_000))
		errMsg := strings.TrimSpace(string(b))
		return fmt.Errorf("cancel job bad response: %s - %s", resp.Status, errMsg)
	}
	return nil
}
func (c client) Put(jobId string, jobPayload runnerlib.JobPayload, skipWebhook bool) error {
	b, err := json.Marshal(jobPayload)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal job payload: %s", err))
	}
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/job/%s", c.trackServerUrl, jobId),
		bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("new put job request: %s", err)
	}
	setAuthHeader(req, c.trackKey)
	if skipWebhook {
		req.Header.Set(SkipWehbooksHeaderName, "true")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("put job: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1_000))
		errMsg := strings.TrimSpace(string(b))
		return fmt.Errorf("put job bad response: %s - %s", resp.Status, errMsg)
	}
	return nil
}
func (c client) Get(jobId string) (TrackJob, runnerlib.JobPayload, bool, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/job/%s", c.trackServerUrl, jobId), nil)
	if err != nil {
		return TrackJob{}, runnerlib.JobPayload{},
			/*isNotFoundErr*/ false, fmt.Errorf("new get job request: %s", err)
	}
	setAuthHeader(req, c.trackKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TrackJob{}, runnerlib.JobPayload{},
			/*isNotFoundErr*/ false, fmt.Errorf("get job: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		isNotFoundErr := resp.StatusCode == http.StatusNotFound
		return TrackJob{}, runnerlib.JobPayload{}, isNotFoundErr, fmt.Errorf("get job bad response: %s", resp.Status)
	}
	var job TrackJob
	err = json.NewDecoder(resp.Body).Decode(&job)
	if err != nil {
		return TrackJob{}, runnerlib.JobPayload{},
			/*isNotFoundErr*/ false, fmt.Errorf("decode job: %s", err)
	}
	var jobPayload runnerlib.JobPayload
	err = json.Unmarshal(job.Payload, &jobPayload)
	if err != nil {
		return TrackJob{}, runnerlib.JobPayload{},
			/*isNotFoundErr*/ false, fmt.Errorf("decode jobPayload: %s", err)
	}
	return job, jobPayload,
		/*isNotFoundErr*/ false, nil
}

func (c client) GetCombinedOutput(jobId string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/job/%s/out", c.trackServerUrl, jobId), nil)
	if err != nil {
		return nil, fmt.Errorf("new get job out request: %s", err)
	}
	setAuthHeader(req, c.trackKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get job out: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("get job out bad response: %s", resp.Status)
	}
	return resp.Body, nil
}

func setAuthHeader(r *http.Request, key string) {
	r.Header.Set(wrappers.TrackKeyHeaderName, key)
}

func parseWebhook(r io.Reader) (job TrackJob, payload runnerlib.JobPayload, err error) {
	err = json.NewDecoder(r).Decode(&job)
	if err != nil {
		err = fmt.Errorf("decode job: %w", err)
		return
	}
	err = json.NewDecoder(bytes.NewBuffer(job.Payload)).Decode(&payload)
	if err != nil {
		err = fmt.Errorf("decode payload: %w", err)
		return
	}
	return
}
