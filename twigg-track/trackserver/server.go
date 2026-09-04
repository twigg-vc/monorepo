// Twigg track: where twigg runners come to run
package trackserver

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"monorepo/base/serverutils"
	"monorepo/buildmeta"
	"monorepo/squeue"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/db"
	"monorepo/twigg-track/handlers/admindash"
	"monorepo/twigg-track/handlers/jobs"
	"monorepo/twigg-track/handlers/testpage"
	"monorepo/twigg-track/runners"
	"monorepo/twigg-track/trackclient"
	"monorepo/twigg-track/wrappers"
	twiggwebclient "monorepo/twigg-web/twigg-web-client"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	_ "modernc.org/sqlite" // register the driver
)

// Struct that configures the server
type SrvConfig struct {
	// Name of the config
	Name string
	// Port the server listens to
	Port int
	// Server full public URL. Examples: https://twigg.vc, http://localhost:9000
	PublicUrl string
	// Absolute path to a directory under which all data is stored
	StorageFolderAbsPath string
	// How long the queue sleeps when there's no message
	JobQueueSleep time.Duration
	// Use to enable custom parameters for the job queue
	UseCustomJobQueueParams bool
	// If UseCustomJobQueueParams, sets the base delay for the queues exponential
	// backoff used to retry messages
	CustomJobQueueBaseRetryDelay time.Duration
	// If UseCustomJobQueueParams, sets the max num of times a message is
	// retried by the job queue before going to the deadletter
	CustomJobQueueMaxRetries int64

	// Number of job runners that run in parallel
	NumJobRunners int32
	// Memory (bytes) allocated for each runner
	JobRunnerMemory int64
	// NanoCpu allocated for each container runner (1e9 = 1 vCPU)
	JobRunnerNanoCpu int64
	// CPU (integer vCPUs) allocated for each VM runner
	JobVmRunnerCpu uint
	// Network bandwidth limit in Mbit/s per VM runner (0 = unlimited)
	JobVmRunnerNetworkMbps uint64
	// VM root disk size in GB (0 = use image default)
	JobVmRunnerDiskGb uint
	// LXD storage pool name for the VM root disk (only used when JobVmRunnerDiskGb > 0)
	JobVmRunnerDiskPool string
	// VM root disk read IOPS limit (0 = unlimited; only used when JobVmRunnerDiskGb > 0)
	JobVmRunnerDiskReadIops uint
	// VM root disk write IOPS limit (0 = unlimited; only used when JobVmRunnerDiskGb > 0)
	JobVmRunnerDiskWriteIops uint
	// How long the queue sleeps when there's no message
	WebhookQueueSleep time.Duration
	// How many payloads the wehbooks queue can run simultaneously
	WebhookQueueConcurrency int
	// Url to which job update notifications (webhooks) will be posted to.
	// Use "" to not post webhooks
	TwiggServerUrl string
	// Path to which the job webhooks will be posted
	TwiggServerWebhookPath string
	// Api key used to authenticate with twigg server.
	// All posted requests will use it as a "TwiggServerKey" header
	TwiggServerKey string
	// Api key required for authentication.
	// All requests to this server must have a header "TrackKey" with this value
	TrackKey string

	// Max QPS
	RateLimitMaxQps float64
	// Max QPS Burst
	RateLimitMaxQpsBurst int

	// If set, DOCKER_HOST env var will be set to OverrideDockerHostTo
	OverrideDockerHost   bool
	OverrideDockerHostTo string

	RequiresGVisor bool // If set, will only run with gvisor
	DontUseGVisor  bool // If set, gvisor wont be used even if available

	// If set, the jobs won't really run - the server will just wait a little
	// and mark them as complete
	MockJobRuns bool

	// If set, all the "running" of jobs will fail; mocking an error with
	// docker/lxd that causes the server to be unale to run jobs
	MockJobCantRun bool

	// If set and if MockJobRuns=false, the runner service will not cleanup
	// containers/VMs/networks when it's initialized
	DisableDockerAndVmsCleanup bool

	// Max acceptable job timeout
	MaxJobTimeout time.Duration
}

// Returns the env var value, or panics if it's unset or empty.
func GetEnvOrDie(envVarName string) string {
	value := os.Getenv(envVarName)
	if value == "" {
		panic(fmt.Sprintf("environment variable %q is required but not set", envVarName))
	}
	return value
}

// Returns the config used for mocket testing - the jobs are not executed in
// containers, but everything else behaves the same way. Use this for testing
// without requiring docker
func MockConfig(Port int, StorageFolderAbsPath string,
	TwiggServerUrl, TwiggServerWebhookPath string, TwiggServerKey string, TrackKey string) SrvConfig {
	return SrvConfig{
		Name:                    "mock",
		Port:                    Port,
		PublicUrl:               fmt.Sprintf("http://localhost:%d", Port),
		StorageFolderAbsPath:    StorageFolderAbsPath,
		JobQueueSleep:           200 * time.Millisecond,
		NumJobRunners:           5,
		WebhookQueueSleep:       200 * time.Millisecond,
		WebhookQueueConcurrency: 5,
		TwiggServerUrl:          TwiggServerUrl,
		TwiggServerWebhookPath:  TwiggServerWebhookPath,
		TwiggServerKey:          TwiggServerKey,
		TrackKey:                TrackKey,
		RateLimitMaxQps:         200,
		RateLimitMaxQpsBurst:    100,
		MockJobRuns:             true,
		MaxJobTimeout:           6 * time.Hour,
	}
}

// Returns the config used for testing
func TestConfig(Port int, StorageFolderAbsPath string,
	TwiggServerUrl, TwiggServerWebhookPath string, TwiggServerKey string, TrackKey string) SrvConfig {
	return SrvConfig{
		Name:                       "test",
		Port:                       Port,
		PublicUrl:                  fmt.Sprintf("http://localhost:%d", Port),
		StorageFolderAbsPath:       StorageFolderAbsPath,
		JobQueueSleep:              200 * time.Millisecond,
		NumJobRunners:              2,
		JobRunnerMemory:            2 * 1024 * 1024 * 1024, // 2 GB
		JobRunnerNanoCpu:           1_000_000_000,          // 1 CPU
		JobVmRunnerCpu:             1,
		JobVmRunnerNetworkMbps:     100,
		JobVmRunnerDiskGb:          15,
		JobVmRunnerDiskPool:        "default",
		JobVmRunnerDiskReadIops:    10000,
		JobVmRunnerDiskWriteIops:   5000,
		WebhookQueueSleep:          200 * time.Millisecond,
		WebhookQueueConcurrency:    5,
		TwiggServerUrl:             TwiggServerUrl,
		TwiggServerWebhookPath:     TwiggServerWebhookPath,
		TwiggServerKey:             TwiggServerKey,
		TrackKey:                   TrackKey,
		RateLimitMaxQps:            200,
		RateLimitMaxQpsBurst:       100,
		DontUseGVisor:              true,
		DisableDockerAndVmsCleanup: true,
		MaxJobTimeout:              6 * time.Hour,
	}
}

// Returns a config for local development
func LocalConfig(Port int, StorageFolderAbsPath string,
	TwiggServerUrl, TwiggServerWebhookPath string, TwiggServerKey string, TrackKey string) SrvConfig {
	return SrvConfig{
		Name:                     "local",
		Port:                     Port,
		PublicUrl:                fmt.Sprintf("http://localhost:%d", Port),
		StorageFolderAbsPath:     StorageFolderAbsPath,
		JobQueueSleep:            5 * time.Second,
		NumJobRunners:            2,
		JobRunnerMemory:          2 * 1024 * 1024 * 1024, // 2 GB
		JobRunnerNanoCpu:         1_000_000_000,          // 1 CPU
		JobVmRunnerCpu:           1,
		JobVmRunnerNetworkMbps:   100,
		JobVmRunnerDiskGb:        15,
		JobVmRunnerDiskPool:      "default",
		JobVmRunnerDiskReadIops:  10000,
		JobVmRunnerDiskWriteIops: 5000,
		WebhookQueueSleep:        5 * time.Second,
		WebhookQueueConcurrency:  5,
		TwiggServerUrl:           TwiggServerUrl,
		TwiggServerWebhookPath:   TwiggServerWebhookPath,
		TwiggServerKey:           TwiggServerKey,
		RateLimitMaxQps:          200,
		RateLimitMaxQpsBurst:     100,
		TrackKey:                 TrackKey,
		MaxJobTimeout:            6 * time.Hour,
	}
}
func HomelabConfig(Port int, StorageFolderAbsPath string,
	TwiggServerUrl, TwiggServerWebhookPath string) SrvConfig {
	return SrvConfig{
		Name:                     "lab",
		Port:                     Port,
		PublicUrl:                fmt.Sprintf("http://localhost:%d", Port),
		StorageFolderAbsPath:     StorageFolderAbsPath,
		JobQueueSleep:            5 * time.Second,
		NumJobRunners:            3,
		JobRunnerMemory:          2 * 1024 * 1024 * 1024, // 2 GB
		JobRunnerNanoCpu:         1_000_000_000,          // 1 CPU
		JobVmRunnerCpu:           1,
		JobVmRunnerNetworkMbps:   100,
		JobVmRunnerDiskGb:        15,
		JobVmRunnerDiskPool:      "default",
		JobVmRunnerDiskReadIops:  10000,
		JobVmRunnerDiskWriteIops: 5000,
		WebhookQueueSleep:        5 * time.Second,
		WebhookQueueConcurrency:  5,
		TwiggServerUrl:           TwiggServerUrl,
		TwiggServerWebhookPath:   TwiggServerWebhookPath,
		TwiggServerKey:           GetEnvOrDie("TWIGG_SERVER_KEY"),
		RateLimitMaxQps:          200,
		RateLimitMaxQpsBurst:     100,
		TrackKey:                 GetEnvOrDie("TWIGG_TRACK_KEY"),
		RequiresGVisor:           true,
		MaxJobTimeout:            6 * time.Hour,
	}
}

// Returns the config used for production
func ProdConfig(Port int, StorageFolderAbsPath string,
	TwiggServerUrl string, TwiggServerWebhookPath string) SrvConfig {
	return SrvConfig{
		Name:                     "prod",
		Port:                     Port,
		PublicUrl:                fmt.Sprintf("http://<WIP>:%d", Port),
		StorageFolderAbsPath:     StorageFolderAbsPath,
		JobQueueSleep:            5 * time.Second,
		NumJobRunners:            3,
		JobRunnerMemory:          2 * 1024 * 1024 * 1024, // 2 GB
		JobRunnerNanoCpu:         1_000_000_000,          // 1 CPU
		JobVmRunnerCpu:           1,
		JobVmRunnerNetworkMbps:   100,
		JobVmRunnerDiskGb:        15,
		JobVmRunnerDiskPool:      "default",
		JobVmRunnerDiskReadIops:  10000,
		JobVmRunnerDiskWriteIops: 5000,
		WebhookQueueSleep:        5 * time.Second,
		WebhookQueueConcurrency:  5,
		TwiggServerUrl:           TwiggServerUrl,
		TwiggServerWebhookPath:   TwiggServerWebhookPath,
		TwiggServerKey:           GetEnvOrDie("TWIGG_SERVER_KEY"),
		TrackKey:                 GetEnvOrDie("TWIGG_TRACK_KEY"),
		RateLimitMaxQps:          200,
		RateLimitMaxQpsBurst:     100,
		RequiresGVisor:           true,
		MaxJobTimeout:            6 * time.Hour,
	}
}

// Main Server.
type Srv struct {
	C              SrvConfig
	TwiggWebClient TwiggWebClient

	// All the following are populated lazily after the server starts running
	isReady    bool
	url        string
	mux        wrappers.Mux
	db         db.Sqlite
	jobQueue   squeue.Runner // Queue for running the jobs
	whQueue    squeue.Runner // Queue for posting webhooks
	isShutDown bool
	shutdownCh chan bool
}

func (s Srv) Url() string {
	if !s.isReady {
		panic("called URL before server was ready")
	}
	return s.url
}
func (s Srv) TrackKey() string {
	return s.C.TrackKey
}

func NewSrv(cfg SrvConfig, twWebClient TwiggWebClient) Srv {
	return Srv{
		C:              cfg,
		TwiggWebClient: twWebClient,
	}
}

type TwiggWebClient interface {
	GetSecretValsFromTwiggWeb(requiredSecretsNames []string, twiggToken string) (requiredSecrets map[string]string, isNotFoundOrForbiddenErr bool, err error)
}

func (s *Srv) Run() {
	if s.C.JobQueueSleep <= 0 {
		panic("JobQueueSleep<=0")
	}
	if s.C.NumJobRunners <= 0 {
		panic("NumJobRunners<=0")
	}
	if s.C.WebhookQueueSleep <= 0 {
		panic("WebhookQueueSleep<=0")
	}
	if s.C.WebhookQueueConcurrency <= 0 {
		panic("WebhookQueueConcurrency<=0")
	}
	if s.C.Name == "prod" && s.C.TrackKey == "" {
		panic("API Key can't be empty for prod")
	}
	if s.C.RateLimitMaxQps == 0 || s.C.RateLimitMaxQpsBurst == 0 {
		panic("RateLimit cant be empty")
	}
	if s.C.MaxJobTimeout <= 0 {
		panic("MaxJobTimeout<=0")
	}
	s.isShutDown = false
	s.shutdownCh = make(chan bool)
	serverutils.AdjustServerDirOrDie(&s.C.StorageFolderAbsPath, "twiggTrackData")

	if s.C.OverrideDockerHost {
		err := os.Setenv("DOCKER_HOST", s.C.OverrideDockerHostTo)
		if err != nil {
			panic(fmt.Sprintf("failed to set docker host to %s: %s",
				s.C.OverrideDockerHostTo, err))
		}
	}

	// Create muxes and clients
	nonRateLimitedMux := http.NewServeMux()

	rateLimittedMux := wrappers.NewRateLimittedMux(s.C.RateLimitMaxQps, s.C.RateLimitMaxQpsBurst, nonRateLimitedMux)
	rateLimittedMux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	s.mux = rateLimittedMux
	authMux := wrappers.NewAuthMux(s.C.TrackKey, s.mux)
	const dbFileName = "track.db"
	var err error
	err = os.MkdirAll(s.C.StorageFolderAbsPath, 0700)
	if err != nil {
		err = fmt.Errorf("failed to mkdir %s: %s", s.C.StorageFolderAbsPath, err)
		return
	}
	absPathToDbFile := filepath.Join(s.C.StorageFolderAbsPath, dbFileName)
	sqliteDb, err := sql.Open(
		"sqlite",
		fmt.Sprintf("file:%s?_pragma=journal_mode=WAL&_pragma=synchronous=NORMAL",
			absPathToDbFile))
	if err != nil {
		err = fmt.Errorf("failed to open db at %s: %s", absPathToDbFile, err)
		return
	}
	err = sqliteDb.Ping()
	if err != nil {
		panic("db is not responding")
	}
	s.db, err = db.NewSqliteFromSql(sqliteDb)
	if err != nil {
		err = fmt.Errorf("failed to create Sqlite db: %s", err)
		return
	}
	err = s.db.Init()
	if err != nil {
		err = fmt.Errorf("failed to init Sqlite db: %s", err)
		return
	}

	jobQueueDb, closeJobQueueDb, err := squeue.NewSqliteStorage(filepath.Join(s.C.StorageFolderAbsPath, "jobqueue"))
	if err != nil {
		fmt.Printf("failed to open queueDb: %s", err)
	}
	if s.C.UseCustomJobQueueParams {
		jobQueueDb.SetBaseRetryDelay(s.C.CustomJobQueueBaseRetryDelay)
		jobQueueDb.SetMaxNumberOfRetries(s.C.CustomJobQueueMaxRetries)
	}
	s.jobQueue = squeue.NewRunner(jobQueueDb, s.C.JobQueueSleep, int(s.C.NumJobRunners))
	whQueueDb, closeWhQueueDb, err := squeue.NewSqliteStorage(filepath.Join(s.C.StorageFolderAbsPath, "whqueue"))
	if err != nil {
		fmt.Printf("failed to open queueDb: %s", err)
	}
	s.whQueue = squeue.NewRunner(whQueueDb, s.C.WebhookQueueSleep, s.C.WebhookQueueConcurrency)

	// Create services
	var rn jobs.RunnersService
	if s.C.MockJobRuns || s.C.MockJobCantRun {
		if s.C.MockJobRuns {
			rn = mockJobRunner{}
			log.Printf("Job executions will be mocked")
		} else {
			rn = mockBadJobRunner{}
			log.Printf("Job executions will be mocked to always fail")
		}
	} else {
		var err error
		if s.C.NumJobRunners <= 0 {
			panic("got NumJobRunners<=0")
		}
		if s.C.JobRunnerMemory <= 0 {
			panic("got JobRunnerMemory<=0")
		}
		if s.C.JobRunnerNanoCpu <= 0 {
			panic("got JobRunnerNanoCpu<=0")
		}
		if s.C.JobVmRunnerCpu == 0 {
			panic("got JobVmRunnerCpu==0")
		}
		const vmRunnerNetworkQuotaBytes = 5 * 1024 * 1024 * 1024 // 5 GB
		// This works best if nRunners = QueueConcurrency
		runnerConfig := runners.NewConfig(
			/*maxParallelJobs*/ s.C.NumJobRunners, // max parallel jobs
			/*maxOutputSize*/ 5*1024*1024, // 5MB max output size
			/*runnerMemoryBytes*/ s.C.JobRunnerMemory,
			/*containerRunnerNanoCpu*/ s.C.JobRunnerNanoCpu, // vCPU per container runner
			/*vmRunnerCpu*/ s.C.JobVmRunnerCpu, // vCPU per VM runner
			/*vmRunnerNetworkMbps*/ s.C.JobVmRunnerNetworkMbps, // network limit per VM runner
			/*vmRunnerDiskGb*/ s.C.JobVmRunnerDiskGb, // VM root disk size
			/*vmRunnerDiskPool*/ s.C.JobVmRunnerDiskPool, // LXD storage pool for VM root disk
			/*vmRunnerDiskReadIops*/ s.C.JobVmRunnerDiskReadIops, // VM root disk read IOPS limit
			/*vmRunnerDiskWriteIops*/ s.C.JobVmRunnerDiskWriteIops, // VM root disk write IOPS limit
			/*vmRunnerNetworkQuotaBytes*/ vmRunnerNetworkQuotaBytes, // VM network limit
			/*disableOutputTTLCleanup*/ false, // enable TTL cleanup
			/*outputTTL*/ 7*24*time.Hour, // 7 days output TTL
			/*outputTTLCleanupInterval*/ 1*time.Hour, // TTL cleanup every hour
			/*logDiskUsageComputeInterval*/ 10*time.Minute, // disk usage computed every 10 mins
			/*maxLogDiskUsage*/ 50*1024*1024*1024, // 50 GB max log disk
		)
		cleanupDockerAndVms := !s.C.DisableDockerAndVmsCleanup
		rn, err = runners.NewService(filepath.Join(s.C.StorageFolderAbsPath, "runners"),
			s.C.RequiresGVisor, s.C.DontUseGVisor, cleanupDockerAndVms, runnerConfig, nil)
		if err != nil {
			panic(fmt.Sprintf("failed to create runner service: %s", err))
		}
		log.Printf("Job runners config:")
		log.Printf("    %d runners will run in parallel", runnerConfig.MaxParallelJobs)
		log.Printf("    %.1f GB memory allocated for each runner -> %.1f GB in total",
			float64(runnerConfig.RunnerMemoryBytes)/(1024*1024*1024),
			float64(runnerConfig.RunnerMemoryBytes*int64(runnerConfig.MaxParallelJobs))/(1024*1024*1024))
		log.Printf("    %.1f CPU allocated for each container runner -> %.1f CPU in total",
			float64(runnerConfig.ContainerRunnerNanoCpu)/(1_000_000_000),
			float64(runnerConfig.ContainerRunnerNanoCpu*int64(runnerConfig.MaxParallelJobs))/(1_000_000_000))
		log.Printf("    %d vCPU allocated for each VM runner, %d Mbit/s network limit, %d GB root disk (pool: %s)",
			runnerConfig.VmRunnerCpu, runnerConfig.VmRunnerNetworkMbps,
			runnerConfig.VmRunnerDiskGb, runnerConfig.VmRunnerDiskPool)
		log.Printf("    %.1f MB of disk allocated to store each output log",
			float64(runnerConfig.MaxOutputSize)/(1024*1024))
		log.Printf("    %.1f GB of disk allocated to store all output logs",
			float64(runnerConfig.MaxLogDiskUsage)/(1024*1024*1024))
		log.Printf("    %d days TTL of output logs", runnerConfig.OutputTTL/(24*time.Hour))
	}

	// Register handlers to the muxes
	authMux.HandleFunc("GET /which-config-please", func(
		// I know this is a silly path, but at least it helps keep some bots away
		w http.ResponseWriter, r wrappers.AuthMuxRequest) {
		w.Write([]byte(s.C.Name))
	})
	testpage.AddHandlers(s.C.PublicUrl, authMux)
	jobs.AddHandlers(s.C.MaxJobTimeout, s.db, rn, s.jobQueue, s.whQueue, s.TwiggWebClient,
		s.C.TwiggServerUrl, s.C.TwiggServerWebhookPath, s.C.TwiggServerKey, authMux)
	admindash.AddHandlers(jobQueueDb, s.jobQueue, authMux)

	// Start the queue and the server
	s.whQueue.Start()
	s.jobQueue.Start()
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(s.C.Port),
		Handler: s.mux,
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		panic(fmt.Sprintf("failed to start listening: %s", err))
	}
	s.url = fmt.Sprintf("http://localhost:%d", s.C.Port)
	go func() {
		log.Printf("TrackServer Build Version: %s\n", buildmeta.Version)
		log.Printf("TrackServer is serving on %s\n", s.url)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
		log.Printf("TrackServer stopped serving on %s\n", s.url)
	}()
	s.isReady = true

	// --- Listen for OS shutdown signals ---
	// signal.NotifyContext creates a context that is canceled on
	// SIGINT or SIGTERM
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop() // Clean up signal handling

	// Block until a shutdown signal is received
	select {
	case <-shutdownCtx.Done():
	case <-s.shutdownCh:
	}
	log.Println("Starting graceful shutdown...")
	s.jobQueue.Stop()
	s.whQueue.Stop()
	_ = sqliteDb.Close()
	_ = closeJobQueueDb()
	_ = closeWhQueueDb()
	rn.Close()

	// --- Create a context with timeout for graceful shutdown ---
	// This ensures HTTP requests and background work don't hang indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // Release resources when shutdown completes

	// --- Shutdown HTTP server gracefully ---
	// Allows in-flight requests to finish within the timeout
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown with error: %q", err)
	}

	s.isShutDown = true
	log.Println("Server shutdown complete.")
}

func (s *Srv) Shutdown() {
	fmt.Println("manual shutdown triggered")
	s.shutdownCh <- true
	for !s.isShutDown {
		time.Sleep(100 * time.Millisecond)
	}
}

// -- Test utils --

func GetTestServerAndFakeWebhookListener(t testing.TB, secrets map[string]string, opts ...func(*SrvConfig)) (Srv, *FakeWebhookListener) {
	wh, whPath := startWebhookListener(t, secrets)
	port, storageFolder := setupTest( /*requiresDocker*/ true, t)
	const testServerTrackKey = "test-key"
	cfg := TestConfig(port, storageFolder, wh.Url(), whPath, wh.twiggServerKey, testServerTrackKey)
	for _, opt := range opts {
		opt(&cfg)
	}
	srv := NewSrv(cfg, &wh)
	go srv.Run()
	for !srv.isReady {
		time.Sleep(10 * time.Microsecond)
	}
	t.Cleanup(srv.Shutdown)
	return srv, &wh
}

// Returns a server that is ready to receive requests and posts webhooks.
// Use the Srv methods to read its URL and TrackKey.
func GetTestServer(twiggServerUrl, twiggServerWebhookPath, twiggServerKey string, t testing.TB) Srv {
	if twiggServerUrl == "" || twiggServerWebhookPath == "" || twiggServerKey == "" {
		t.Fatal("misuse of GetTestServer - use GetNoWebhookTestServer instead")
	}
	port, storageFolder := setupTest( /*requiresDocker*/ true, t)
	const testServerTrackKey = "test-key"
	cfg := TestConfig(port, storageFolder, twiggServerUrl, twiggServerWebhookPath, twiggServerKey, testServerTrackKey)
	twiggWebClient := twiggwebclient.NewClient(cfg.TwiggServerUrl, cfg.TwiggServerKey)
	srv := NewSrv(cfg, twiggWebClient)
	go srv.Run()
	for !srv.isReady {
		time.Sleep(10 * time.Microsecond)
	}
	t.Cleanup(srv.Shutdown)
	return srv
}

// Returns a webserver ready to receive requests and that doesn't post webhooks.
// Use the Srv methods to read its URL and TrackKey.
func GetNoWebhookTestServer(t testing.TB, options ...serverOption) Srv {
	port, storageFolder := setupTest( /*requiresDocker*/ true, t)
	const testServerTrackKey = "test-key"
	cfg := TestConfig(port, storageFolder, "", "", "", testServerTrackKey)
	for _, option := range options {
		option(&cfg)
	}
	twiggWebClient := twiggwebclient.NewClient(cfg.TwiggServerUrl, cfg.TwiggServerKey)
	srv := NewSrv(cfg, twiggWebClient)
	go srv.Run()
	for !srv.isReady {
		time.Sleep(10 * time.Microsecond)
	}
	t.Cleanup(srv.Shutdown)
	return srv
}

// Returns a server ready to receive requests. It uses a mocked runner - i.e.
// the jobs are not actually executed in containers; but everything else is
// the same
func GetMockTestServer(twiggServerUrl, twiggServerWebhookPath, twiggServerKey string, t testing.TB) Srv {
	port, storageFolder := setupTest( /*requiresDocker*/ false, t)
	const testServerTrackKey = "test-key"
	cfg := MockConfig(port, storageFolder, twiggServerUrl, twiggServerWebhookPath,
		twiggServerKey, testServerTrackKey)
	twiggWebClient := twiggwebclient.NewClient(cfg.TwiggServerUrl, cfg.TwiggServerKey)
	srv := NewSrv(cfg, twiggWebClient)
	go srv.Run()
	for !srv.isReady {
		time.Sleep(10 * time.Microsecond)
	}
	t.Cleanup(srv.Shutdown)
	return srv
}

// Returns false if docker - a requirement for the server - is installed
func DockerIsInstalled() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// Requires docker to be installed.
// Get a port and serverStorageDir that can be used to instantiate
// and start a server. Automatically sets up the cleanups.
func setupTest(requiresDocker bool, t testing.TB) (port int, serverStorageDir string) {
	t.Helper()
	if requiresDocker && !DockerIsInstalled() {
		panic("docker is not installed")
	}
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to create test instance: %s", err)
	}
	serverStorageDir = filepath.Join(currentDir, "track-main-test")
	os.MkdirAll(serverStorageDir, os.ModePerm)
	os.RemoveAll(serverStorageDir)
	t.Cleanup(
		func() {
			os.RemoveAll(serverStorageDir)
		},
	)
	port = serverutils.GetFreePort(t)
	return
}

// Starts a local server to receive the job update notifications
func startWebhookListener(t testing.TB, secrets map[string]string) (ns FakeWebhookListener, webhookPath string) {
	jobUpdates := []trackclient.TrackJob{}
	const twiggServerKey = "fake-twigg-server-key"
	port := serverutils.GetFreePort(t)
	webhookPath = "/"
	ns = FakeWebhookListener{
		updates:              &jobUpdates,
		url:                  fmt.Sprintf("http://localhost:%d", port),
		twiggServerKey:       twiggServerKey,
		secretsNamesToValues: secrets,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("PUT "+webhookPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("TwiggServerKey") != twiggServerKey {
			http.Error(w, "bad TwiggServerKey", http.StatusUnauthorized)
			return
		}
		defer r.Body.Close()
		job, _, err := trackclient.ParseWebhook(r.Body)
		if err != nil {
			http.Error(w, "bad webhook body", http.StatusUnauthorized)
			return
		}
		jobUpdates = append(jobUpdates, job)
	})
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	mockNotifyServer := &http.Server{
		Handler: mux,
	}
	go func() {
		if err := mockNotifyServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			t.Fatal(err)
		}
	}()
	t.Cleanup(func() { mockNotifyServer.Close() })
	return
}

// Lightweight fake implementation of a twigg-server that receives wehbooks
type FakeWebhookListener struct {
	updates                 *[]trackclient.TrackJob
	url                     string
	twiggServerKey          string
	secretsNamesToValues    map[string]string
	getSecretLastCalledWith []string
}

func (ns FakeWebhookListener) GetSecretLastCalledWith() []string {
	return ns.getSecretLastCalledWith
}

// Waits until a webhook of a status of a specifid job is received
func (ns FakeWebhookListener) WaitFor(jobId string, status trackclient.TrackJobStatus, t *testing.T) {
	start := time.Now()
	for {
		found := false
		for i := range *ns.updates {
			if (*ns.updates)[i].Id == jobId && (*ns.updates)[i].Status == status {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if time.Since(start) > 10*time.Second {
			t.Fatalf("waited too long for webhook %s for jobId %s", status, jobId)
		}
	}
}
func (ns FakeWebhookListener) Url() string {
	return ns.url
}
func (ns *FakeWebhookListener) GetSecretValsFromTwiggWeb(requiredSecretsNames []string, twiggToken string) (map[string]string, bool, error) {
	ns.getSecretLastCalledWith = requiredSecretsNames
	if ns.secretsNamesToValues == nil {
		ns.secretsNamesToValues = map[string]string{}
	}
	requiredSecrets := make(map[string]string)
	for _, secret := range requiredSecretsNames {
		secretVal, contains := ns.secretsNamesToValues[secret]
		if !contains {
			return nil, true, fmt.Errorf("secret %q not found", secret)
		}
		requiredSecrets[secret] = secretVal
	}
	return requiredSecrets, false, nil
}
func (ns *FakeWebhookListener) GetNotificationsCount() int {
	return len(*ns.updates)
}

// Mocks the service that runs jobs and always succeeds
type mockJobRunner struct{}

func (m mockJobRunner) Run(jobId string, j runnerlib.JobPayload) (trackclient.TrackJobStatus, error) {
	return trackclient.TrackJobStatusSuccess, nil
}
func (m mockJobRunner) Read(jobId string) (out io.ReadCloser, isNotFoundErr bool, err error) {
	out = io.NopCloser(bytes.NewBufferString("mocked out"))
	isNotFoundErr = false
	err = nil
	return
}
func (m mockJobRunner) Cancel(jobId string) {}
func (m mockJobRunner) Close()              {}

// Mocks a job runner with some runtime error that causes all Run calls to err
type mockBadJobRunner struct{}

func (m mockBadJobRunner) Run(jobId string, j runnerlib.JobPayload) (trackclient.TrackJobStatus, error) {
	return "", errors.New("mock err")
}
func (m mockBadJobRunner) Read(jobId string) (out io.ReadCloser, isNotFoundErr bool, err error) {
	isNotFoundErr = true
	err = errors.New("not found")
	return
}
func (m mockBadJobRunner) Cancel(jobId string) {}
func (m mockBadJobRunner) Close()              {}

type serverOption func(c *SrvConfig)
