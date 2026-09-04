package trackclient

import (
	"io"
	"monorepo/twigg-runner/runnerlib"
	"net/http"
)

// MUST BE INITIALIZED WITH `NewClient`
// Client for services to communicate with the track server
type Client struct {
	client
}

// Instantiates a new Client ready to be used
func NewClient(trackServerUrl string, trackKey string) Client {
	return Client{
		client: client{
			trackServerUrl: trackServerUrl,
			trackKey:       trackKey,
			httpClient:     &http.Client{},
		},
	}
}

// Puts (idempotent) a job with a runnerlib payload format so that it starts running
func (c Client) Put(jobId string, jobPayload runnerlib.JobPayload) error {
	return c.client.Put(jobId, jobPayload,
		/*skipWebhook*/ false)
}

// Same as Put, but asks the server to not send any webhooks for this job
func (c Client) PutSkipWebhook(jobId string, jobPayload runnerlib.JobPayload) error {
	return c.client.Put(jobId, jobPayload, true)
}

// Requests for a job to be canceled. Only returns error if the requests itself
// fails. I.e. it also returns "nil" if there were no jobs running with that id
func (c Client) Cancel(jobId string) error {
	return c.client.Cancel(jobId)
}

// Returns the combined of the job
func (c Client) GetCombinedOutput(jobId string) (io.ReadCloser, error) {
	return c.client.GetCombinedOutput(jobId)
}

// Checks if the server health is ok.
// Always returns either `true, nil` or `false, <non nil>`.
func (c Client) HealthIsOk() (bool, error) {
	return c.client.HealthIsOk()
}

// Returns the job data and the payload. Note that runnerlib.JobPayload is
// simply the `jobservice.Job` decoded.
func (c Client) Get(jobId string) (tj TrackJob, pl runnerlib.JobPayload, isNotFoundErr bool, err error) {
	return c.client.Get(jobId)
}

// Parses a webhook that was posted by the track server
func ParseWebhook(r io.Reader) (TrackJob, runnerlib.JobPayload, error) {
	return parseWebhook(r)
}

type TrackJob struct {
	Id                  string
	Payload             []byte
	SkipWebhooks        bool // if set, webhooks are not sent
	Status              TrackJobStatus
	CreatedAtMillis     int64 // unix milliseconds
	FinalDurationMillis int64 // how long in ms the job took to run (only set for status=done)
}

type TrackJobStatus string

const (
	TrackJobStatusQueued  TrackJobStatus = "queued"
	TrackJobStatusRunning TrackJobStatus = "running"
	TrackJobStatusSuccess TrackJobStatus = "success"
	TrackJobStatusFail    TrackJobStatus = "fail"
	TrackJobStatusTimeout TrackJobStatus = "timeout"
	TrackJobStatusCancel  TrackJobStatus = "cancel"
)

const SkipWehbooksHeaderName = "SkipWebhooks"
