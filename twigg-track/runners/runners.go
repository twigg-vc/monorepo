package runners

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	lxd "github.com/canonical/lxd/client"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/moby/moby/client"
)

type service struct {
	cfg                        Config
	docker                     *client.Client     // nil when docker is not available
	lxdServer                  lxd.InstanceServer // nil when lxd is not available
	canRunDockerJobs           bool
	canRunVmJobs               bool
	shouldUseGVisor            bool
	autoCleanupDockerAndLxd    bool
	absPathToWorkdir           string
	workdirSize                atomic.Int64
	workdirSizeComputerStopCh  chan bool
	computeWorkdirSizeRunCount atomic.Int32
	nJobsRunning               atomic.Int32
	closeWaitGroup             sync.WaitGroup
	ttlJanitorRunCount         atomic.Int32
	ttlJanitorStopCh           chan bool

	cancelMu          sync.Mutex
	jobIdToCancelFunc map[string]func()
}

// Label/val used for containers and networks created by this service
const (
	dockerLabelKey = "twigg-track-runner"
	dockerLabelVal = "true"
)

func newService(absPathToWorkdir string, requiresGVisor bool, dontUseGVisor bool,
	autoCleanupContainersAndVmsOnInitAndClose bool, cfg Config,
	checkCanRunOverride func() (canRunDockerJobs bool, canRunVmJobs bool, err error)) (*service, error) {
	if cfg.MaxParallelJobs <= 0 {
		panic("got MaxParallelRunners<=0")
	}
	if cfg.MaxOutputSize <= 0 {
		panic("got MaxOutputSize<=0")
	}
	if cfg.RunnerMemoryBytes <= 0 {
		panic("got RunnerMemory<=0")
	}
	if cfg.ContainerRunnerNanoCpu <= 0 {
		panic("got ContainerRunnerNanoCpu<=0")
	}
	if cfg.OutputTTL <= 0 {
		panic("got OutputTTL<=0")
	}
	if cfg.OutputTTLCleanupInterval <= 0 {
		panic("got OutputTTLCleanupInterval<=0")
	}
	if cfg.LogDiskUsageComputeInterval <= 0 {
		panic("got LogDiskUsageComputeInterval<=0")
	}
	if cfg.MaxLogDiskUsage <= 0 {
		panic("got MaxOutputDiskUsage<=0")
	}
	if cfg.VmRunnerCpu == 0 {
		panic("got VmRunnerCpu==0")
	}
	if cfg.VmRunnerDiskGb > 0 && cfg.VmRunnerDiskPool == "" {
		panic("got VmRunnerDiskGb>0 but VmRunnerDiskPool is empty")
	}
	var canRunDockerJobs bool
	var canRunVmJobs bool
	var err error
	if checkCanRunOverride != nil {
		canRunDockerJobs, canRunVmJobs, err = checkCanRunOverride()
	} else {
		canRunDockerJobs, canRunVmJobs, err = CheckCanRun()
	}
	if err != nil {
		return nil, fmt.Errorf("determine which jobs can run: %s", err)
	}

	var docker *client.Client
	var shouldUseGVisor bool
	if canRunDockerJobs {
		docker, shouldUseGVisor, err = newDockerClient(requiresGVisor, dontUseGVisor)
		if err != nil {
			return nil, fmt.Errorf("create docker client: %s", err)
		}
		if autoCleanupContainersAndVmsOnInitAndClose {
			if err := cleanupContainersAndNetworks(docker); err != nil {
				return nil, err
			}
		}
	}

	var lxdServer lxd.InstanceServer
	if canRunVmJobs {
		lxdServer, err = lxd.ConnectLXDUnix("", nil)
		if err != nil {
			return nil, fmt.Errorf("create lxc client: %s", err)
		}
		if autoCleanupContainersAndVmsOnInitAndClose {
			if err := cleanupLxdInstances(lxdServer); err != nil {
				return nil, err
			}
		}
		// Always recreate the shared network and ACL so any config changes in the
		// binary take effect immediately. Instance cleanup above ensures no VMs are
		// attached before we tear the network down.
		if err := recreateLxdNetworkAndACL(lxdServer,
			autoCleanupContainersAndVmsOnInitAndClose); err != nil {
			return nil, err
		}
	}
	_ = os.MkdirAll(absPathToWorkdir, 0700)
	size, err := computeworkdirDirSize(absPathToWorkdir)
	if err != nil {
		return nil, err
	}

	srv := &service{
		cfg:                       cfg,
		absPathToWorkdir:          absPathToWorkdir,
		docker:                    docker,
		lxdServer:                 lxdServer,
		canRunDockerJobs:          canRunDockerJobs,
		canRunVmJobs:              canRunVmJobs,
		shouldUseGVisor:           shouldUseGVisor,
		autoCleanupDockerAndLxd:   autoCleanupContainersAndVmsOnInitAndClose,
		closeWaitGroup:            sync.WaitGroup{},
		workdirSizeComputerStopCh: make(chan bool, 1),
		ttlJanitorStopCh:          make(chan bool, 1),

		jobIdToCancelFunc: map[string]func(){},
	}
	srv.workdirSize.Store(size)
	srv.computeWorkdirSizeRunCount.Add(1)

	if !srv.cfg.DisableOutputTTLCleanup {
		srv.startTTLJanitor()
	}
	srv.startWorkdirSizeComputer()
	return srv, nil
}
func (s *service) enableOutputTTLCleanup() {
	if !s.cfg.DisableOutputTTLCleanup {
		// Already started in the constructor
		return
	}
	s.startTTLJanitor()
}

func (s *service) Close() {
	s.cancelAllJobs()
	start := time.Now()
	for s.nJobsRunning.Load() != 0 {
		time.Sleep(200 * time.Millisecond)
		if time.Since(start) > 30*time.Second {
			break
		}
	}
	dockerCleanupCh := make(chan bool, 1)
	go func() {
		if s.autoCleanupDockerAndLxd {
			if s.docker != nil {
				_ = cleanupContainersAndNetworks(s.docker)
			}
			if s.lxdServer != nil {
				_ = cleanupLxdInstances(s.lxdServer)
			}
		}
		dockerCleanupCh <- true
	}()
	if s.ttlJanitorStopCh != nil {
		s.ttlJanitorStopCh <- true
	}
	if s.workdirSizeComputerStopCh != nil {
		s.workdirSizeComputerStopCh <- true
	}
	s.closeWaitGroup.Wait()
	<-dockerCleanupCh
	if s.docker != nil {
		_ = s.docker.Close()
	}
}

func cleanupContainersAndNetworks(docker *client.Client) error {
	cleanupCtx, deleteCleanupCtx := context.WithTimeout(
		context.Background(),
		30*time.Second)
	defer deleteCleanupCtx()
	cs, err := docker.ContainerList(cleanupCtx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", dockerLabelKey+"="+dockerLabelVal),
		),
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %s", err)
	}
	for _, c := range cs {
		err = killAndRemoveContainer(docker, c.ID)
		if err != nil {
			return err
		}
	}
	nws, err := docker.NetworkList(cleanupCtx, network.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", dockerLabelKey+"="+dockerLabelVal),
		),
	})
	if err != nil {
		return err
	}
	for _, nw := range nws {
		err = removeNetwork(docker, nw.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// Helper to synchronously remove a container
func killAndRemoveContainer(docker *client.Client, containerId string) error {
	// Get channels for waiting for removal
	timeoutCtx, cancelTimeoutCtx := context.WithTimeout(
		context.Background(), 2*time.Second)
	defer cancelTimeoutCtx()
	waitUntilRemovedCh, waitUntilRemovedErrCh := docker.ContainerWait(
		timeoutCtx,
		containerId,
		container.WaitConditionRemoved,
	)
	// Trigger the removal using the background ctx
	err := docker.ContainerRemove(
		timeoutCtx,
		containerId,
		container.RemoveOptions{Force: true},
	)
	if cerrdefs.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	select {
	case <-timeoutCtx.Done():
		return fmt.Errorf("timeout removing containerId=%q", containerId)
	case <-waitUntilRemovedCh:
		return nil
	case err := <-waitUntilRemovedErrCh:
		if cerrdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
}

func removeNetwork(docker *client.Client, networkID string) error {
	timeoutCtx, cancelTimeoutCtx := context.WithTimeout(
		context.Background(), 2*time.Second)
	defer cancelTimeoutCtx()
	rmErr := docker.NetworkRemove(timeoutCtx, networkID)
	if cerrdefs.IsNotFound(rmErr) {
		return nil
	}
	return rmErr
}

// Only returns an error if it waits for too long
func waitUntilNetworkIsUp(docker *client.Client, networkName string) error {
	timeoutCtx, cancelTimeoutCtx := context.WithTimeout(
		context.Background(), 2*time.Second)
	defer cancelTimeoutCtx()
	backoff := 10 * time.Millisecond
	maxBackoff := 200 * time.Millisecond
	for {
		_, err := docker.NetworkInspect(timeoutCtx, networkName, network.InspectOptions{})
		if err == nil {
			return nil
		}
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("spent too long waiting for net=%s to be up", networkName)
		case <-time.After(backoff):
			backoff *= 2 // Exponential backoff
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func getJobIdHash(jobId string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(jobId)))
}

const jobFolderPathPrefix = "runner-job-hash-"

func (s *service) jobFolderPath(jobIdHash string) string {
	return filepath.Join(s.absPathToWorkdir, jobFolderPathPrefix+jobIdHash)
}

func (s *service) cancel(jobId string) {
	s.cancelMu.Lock()
	cancelFunc, ok := s.jobIdToCancelFunc[jobId]
	s.cancelMu.Unlock()

	if ok && cancelFunc != nil {
		cancelFunc()
	}
}

func (s *service) cancelAllJobs() {
	s.cancelMu.Lock()
	for _, cancelFunc := range s.jobIdToCancelFunc {
		cancelFunc()
	}
	s.cancelMu.Unlock()
}

func (s *service) run(jobId_ string, j runnerlib.JobPayload) (st trackclient.TrackJobStatus, err error) {
	if s.workdirSize.Load() >= s.cfg.MaxLogDiskUsage {
		err = ErrOutputDiskIsFull
		return
	}
	shouldRunInDocker := false
	shouldRunInLxd := false
	if j.ImageName == runnerlib.VmImage {
		shouldRunInLxd = true
	} else {
		if slices.Contains(runnerlib.SupportedImages, j.ImageName) {
			shouldRunInDocker = true
		}
	}
	if !shouldRunInDocker && !shouldRunInLxd {
		err = fmt.Errorf("unsupported image %s", j.ImageName)
		return
	}

	if shouldRunInDocker && !s.canRunDockerJobs {
		err = ErrDockerNotAvailable
		return
	}
	if shouldRunInLxd && !s.canRunVmJobs {
		err = ErrVmNotAvailable
		return
	}

	manualCancelCtx, cancelManualCtx, err := s.createCancelFuncIfJobNotRunning(jobId_)
	if err != nil {
		return
	}
	defer func() {
		s.cancelMu.Lock()
		cancelManualCtx()
		delete(s.jobIdToCancelFunc, jobId_)
		s.cancelMu.Unlock()
	}()

	err = s.incrementNumberOfJobsRunningIfLessThanMax()
	if err != nil {
		return
	}
	defer s.nJobsRunning.Add(-1)

	// Get everything required for the job to be run
	jobIdHash := getJobIdHash(jobId_)
	path := s.jobFolderPath(jobIdHash)
	combinedOutFile, err := s.getCombinedOutFile(path)
	if err != nil {
		return
	}
	defer combinedOutFile.Close()
	outWriter, errWriter := s.getOutputWriters(combinedOutFile)
	payload, err := json.Marshal(j)
	if err != nil {
		return
	}

	// Start running the job
	runCount := 0
	if shouldRunInDocker {
		st, err = s.runJobInDocker(jobId_, jobIdHash, path, j, payload, outWriter, errWriter, manualCancelCtx)
		if err != nil {
			return
		}
		runCount += 1
	}
	if shouldRunInLxd {
		st, err = s.runJobInLxd(jobId_, jobIdHash, path, j, payload, outWriter, errWriter, manualCancelCtx)
		if err != nil {
			return
		}
		runCount += 1
	}
	if runCount != 1 {
		panic(fmt.Sprintf("job ran %d times", runCount))
	}

	// Sync the file to ensure their durability. Ignore errors.
	_ = combinedOutFile.Sync()
	return
}

// Creates a context for canceling a job that is running and puts its
// cancelation function in the jobIdToCancelFunc so that cancel(jobId)
// can cancel it. Only returns an error if there already is a
// job with that id running.
func (s *service) createCancelFuncIfJobNotRunning(jobId string) (context.Context, context.CancelFunc, error) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()

	_, ok := s.jobIdToCancelFunc[jobId]
	if ok {
		return nil, nil, fmt.Errorf("%s already running", jobId)
	}
	manualCancelCtx, cancelManualCtx := context.WithCancel(context.Background())
	s.jobIdToCancelFunc[jobId] = cancelManualCtx
	return manualCancelCtx, cancelManualCtx, nil
}

// Atomically increment the number of jobs running
// Returns an error only if there are too many jobs running
func (s *service) incrementNumberOfJobsRunningIfLessThanMax() error {
	for {
		cur := s.nJobsRunning.Load()
		if cur >= s.cfg.MaxParallelJobs {
			return ErrTooManyJobs
		}
		if s.nJobsRunning.CompareAndSwap(cur, cur+1) {
			break
		}
	}
	return nil
}

func (s *service) getCombinedOutFile(jobFolderPath string) (*os.File, error) {
	err := os.MkdirAll(jobFolderPath, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create job dir: %w", err)
	}
	combinedOutFile, err := os.Create(filepath.Join(jobFolderPath, combinedOutFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to create err file: %w", err)
	}
	return combinedOutFile, nil
}

func (s *service) getOutputWriters(combinedOutFile *os.File) (outWriter, errWriter *combinedOutputWriter) {
	written := int64(0)
	mu := sync.Mutex{}
	outWriter = &combinedOutputWriter{
		combinedOutput: combinedOutFile,
		prefix:         StdOutPrefix,
		totalLimit:     s.cfg.MaxOutputSize,
		totalWritten:   &written,
		sharedMu:       &mu,
	}
	errWriter = &combinedOutputWriter{
		combinedOutput: combinedOutFile,
		prefix:         StdErrPrefix,
		totalLimit:     s.cfg.MaxOutputSize,
		totalWritten:   &written,
		sharedMu:       &mu,
	}
	return
}

func logOutput(start time.Time,
	isTimeoutExceeded bool, isManualCancel bool, isNetworkQuotaExceeded bool,
	isSuccess bool, isOOM bool,
	isSigkill bool, isRuntimeErr bool,
	failureExitCode int64,
	outWriter io.Writer) (trackclient.TrackJobStatus, error) {
	switch {
	case isTimeoutExceeded:
		_, err := fmt.Fprintf(outWriter, "\nJOB TIMED OUT AFTER %s\n", time.Since(start))
		return trackclient.TrackJobStatusTimeout, err

	case isManualCancel:
		_, err := fmt.Fprintf(outWriter, "\nJOB CANCELED AFTER %s\n", time.Since(start))
		return trackclient.TrackJobStatusCancel, err

	case isNetworkQuotaExceeded:
		_, err := fmt.Fprintf(outWriter, "\nJOB FAILED AFTER %s: EXCEEDED NETWORK QUOTA\n", time.Since(start))
		return trackclient.TrackJobStatusFail, err

	case isSuccess:
		_, err := fmt.Fprintf(outWriter, "\nJOB SUCCEEDED AFTER %s :)\n", time.Since(start))
		return trackclient.TrackJobStatusSuccess, err

	case isOOM:
		_, err := fmt.Fprintf(outWriter, "\nJOB FAILED AFTER %s: EXCEEDED MEMORY LIMITS\n", time.Since(start))
		return trackclient.TrackJobStatusFail, err

	case isSigkill:
		_, err := fmt.Fprintf(outWriter, "\nJOB FAILED AFTER %s: TERMINATED BY SIGKILL\n", time.Since(start))
		return trackclient.TrackJobStatusFail, err

	case isRuntimeErr:
		_, err := fmt.Fprintf(outWriter, "\nJOB FAILED AFTER %s: RUNTIME ERROR\n", time.Since(start))
		return trackclient.TrackJobStatusFail, err

	default:
		_, err := fmt.Fprintf(outWriter, "\nJOB FAILED AFTER %s: EXIT CODE = %d\n", time.Since(start), failureExitCode)
		return trackclient.TrackJobStatusFail, err
	}
}

func (s *service) read(jobId string) (out io.ReadCloser, isNotFoundErr bool, err error) {
	path := s.jobFolderPath(getJobIdHash(jobId))
	out, err = os.Open(filepath.Join(path, combinedOutFileName))
	if err != nil {
		isNotFoundErr = os.IsNotExist(err)
		err = fmt.Errorf("failed to open out.log: %s", err)
		return
	}
	return
}

func (s *service) startTTLJanitor() {
	s.closeWaitGroup.Add(1)
	go func() {
		defer s.closeWaitGroup.Done()

		ticker := time.NewTicker(s.cfg.OutputTTLCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.deleteTTLdOutputs()
			case <-s.ttlJanitorStopCh:
				return
			}
		}
	}()
}

// This is a "best effor" method: if it fails to delete entries, it simply logs errors
func (s *service) deleteTTLdOutputs() {
	rootDir := s.absPathToWorkdir
	cutoff := time.Now().Add(-s.cfg.OutputTTL)
	// First pass: delete old files
	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fileDirHasPrefix := strings.HasPrefix(filepath.Base(filepath.Dir(path)), jobFolderPathPrefix)
		if info.ModTime().Before(cutoff) &&
			fileDirHasPrefix {
			err := os.Remove(path)
			if err == nil {
				log.Printf("successfully TTL'd file %s", path)
			}
		}
		return nil
	})
	// Second pass: remove empty directories bottom-up
	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == rootDir {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err == nil && len(entries) == 0 && strings.HasPrefix(filepath.Base(path),
			jobFolderPathPrefix) {
			_ = os.Remove(path)
		}
		return nil
	})
	s.ttlJanitorRunCount.Add(1)
}

func (s *service) startWorkdirSizeComputer() {
	s.closeWaitGroup.Add(1)
	go func() {
		defer s.closeWaitGroup.Done()
		ticker := time.NewTicker(s.cfg.LogDiskUsageComputeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				size, err := computeworkdirDirSize(s.absPathToWorkdir)
				s.computeWorkdirSizeRunCount.Add(1)
				if err != nil {
					log.Printf("failed to compute workdir size: %s", err)
					continue
				}
				s.workdirSize.Store(size)
			case <-s.workdirSizeComputerStopCh:
				return
			}
		}
	}()
}

func newDockerClient(requiresGVisor bool, dontUseGVisor bool) (*client.Client, bool, error) {
	docker, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create docker client: %w", err)
	}
	_, err = docker.Ping(context.Background())
	if err != nil {
		return nil, false, fmt.Errorf("docker is not responding: %w", err)
	}
	gvisorIsAvailable := false
	info, err := docker.Info(context.Background())
	if err != nil {
		return nil, false, fmt.Errorf("failed to get docker info: %w", err)
	}
	for name := range info.Runtimes {
		if name == "runsc" {
			gvisorIsAvailable = true
			break
		}
	}
	if requiresGVisor && !gvisorIsAvailable {
		return nil, false, fmt.Errorf("config requires gvisor but it's not available")
	}
	shouldUseGVisor := gvisorIsAvailable && !dontUseGVisor
	if shouldUseGVisor {
		log.Printf("will use gvisor for containers\n")
	}
	return docker, shouldUseGVisor, nil
}

func computeworkdirDirSize(absPathToWorkdir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(absPathToWorkdir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip dirs and weird files (symlinks, and
		// possibly other stuff - I'm not sure).
		// symlinks should not exists in that folder but we'll keep to be safe
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to compute workdir size: %s", err)
	}
	return total, nil
}

const (
	combinedOutFileName = "combined_out.log"
	resolvConfFileName  = "resolv.conf"
)

type combinedOutputWriter struct {
	combinedOutput *os.File
	prefix         string
	totalLimit     int64
	totalWritten   *int64
	sharedMu       *sync.Mutex
	limitReached   bool
}

func (w *combinedOutputWriter) Write(p []byte) (n int, err error) {
	w.sharedMu.Lock()
	defer w.sharedMu.Unlock()

	// If we are already over the limit, just pretend we wrote it
	if w.limitReached {
		return len(p), nil
	}

	// Check if this specific write will push us over
	if *w.totalWritten+int64(len(p)) > w.totalLimit {
		// Ignore these bytes requested and just write a msg
		_, err = w.combinedOutput.Write([]byte("\n--- [LOG LIMIT REACHED: OUTPUT TRUNCATED] ---\n"))
		if err != nil {
			return 0, err
		}
		w.limitReached = true
		return len(p), nil
	}
	formatted := fmt.Sprintf("%s%s", w.prefix, string(p))
	n_actual, err := w.combinedOutput.Write([]byte(formatted))
	*w.totalWritten += int64(n_actual)
	return len(p), err
}

// Helper to run a cleanup function and retry a few times on error.
// If errors persist even after a few retries, it logs the error.
func runCleanup(cleanupName string, cleanupFunc func() error) {
	const retries = 10
	const backoff = 200 * time.Millisecond
	var err error
	for i := 0; i < retries; i++ {
		err = cleanupFunc()
		if err == nil {
			return
		}
		time.Sleep(backoff)
	}
	log.Printf("%s failed after %d attempts: %v", cleanupName, retries, err)
}
