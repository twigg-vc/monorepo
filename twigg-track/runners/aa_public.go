package runners

import (
	"errors"
	"io"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"time"
)

// Service for creating VMs with runners inside them
type Service struct {
	s *service
}

// If autoCleanupContainersAndVmsOnInitAndClose, VMs, docker containers and networks
// are cleaned-up when NewService is called and when Close is called.
// If requiresGVisor, returns an error when gVisor is not available.
// If dontUseGVisor, gVisor is never used even if available.
// checkCanRunOverride, if not nil, is used to determine what can/cant run
func NewService(absPathToWorkdir string, requiresGVisor bool, dontUseGVisor bool,
	autoCleanupContainersAndVmsOnInitAndClose bool, cfg Config,
	checkCanRunOverride func() (canRunDockerJobs bool, canRunVmJobs bool, err error)) (Service, error) {
	s, err := newService(absPathToWorkdir, requiresGVisor,
		dontUseGVisor, autoCleanupContainersAndVmsOnInitAndClose,
		cfg, checkCanRunOverride)
	return Service{
		s: s,
	}, err
}

// Call when done using
func (s Service) Close() {
	s.s.Close()
}

// Runs the job respecting its timeout
func (s Service) Run(jobId string, j runnerlib.JobPayload) (trackclient.TrackJobStatus, error) {
	return s.s.run(jobId, j)
}

// Request for a job to be cancelled.
// Does nothing it a running job is not found.
func (s Service) Cancel(jobId string) {
	s.s.cancel(jobId)
}

// Read the combined output of a job
func (s Service) Read(jobId string) (out io.ReadCloser, isNotFoundErr bool, err error) {
	return s.s.read(jobId)
}

// Returns the number of jobs activelly running
func (s Service) JobsRunning() int32 {
	return s.s.nJobsRunning.Load()
}

// Returns the disk space used to store output logs
func (s Service) OutputLogsDiskUsage() int64 {
	return s.s.workdirSize.Load()
}

// Returns the number of times the output logs disk usage was computed
func (s Service) OutputLogsDiskUsageComputationCount() int32 {
	return s.s.computeWorkdirSizeRunCount.Load()
}

// If DisableOutputTTLCleanup=true was used, this method enables it
func (s Service) EnableOutputTTLCleanup() {
	s.s.enableOutputTTLCleanup()
}

// Returns the number of times TTL cleanups were executed
func (s Service) OutputTTLCleanupCount() int32 {
	return s.s.ttlJanitorRunCount.Load()
}

// Returns booleans that indicate which kind of jobs the server can run
func (s Service) GetCanRun() (canRunDockerJobs bool, canRunVmJobs bool) {
	return s.s.canRunDockerJobs, s.s.canRunVmJobs
}

type Config struct {
	MaxParallelJobs             int32         // Max number of jobs running in parallel
	MaxOutputSize               int64         // Max size (bytes) of the output log
	RunnerMemoryBytes           int64         // Memory (bytes) allocated to each runner
	ContainerRunnerNanoCpu      int64         // vCPU allocated per container runner (e.g. 500_000_000 for 0.5 CPU)
	VmRunnerCpu                 uint          // CPU allocated per VM runner (e.g. 1 for 1 CPU)
	VmRunnerNetworkMbps         uint64        // Network bandwidth limit in Mbit/s per VM runner (0 = unlimited; VM only)
	VmRunnerDiskGb              uint          // VM root disk size in GB (0 = use image default; VM only)
	VmRunnerDiskPool            string        // LXD storage pool name for the VM root disk (only used when VmRunnerDiskGb > 0)
	VmRunnerDiskReadIops        uint          // VM root disk read IOPS limit (0 = unlimited; VM only; only used when VmRunnerDiskGb > 0)
	VmRunnerDiskWriteIops       uint          // VM root disk write IOPS limit (0 = unlimited; VM only; only used when VmRunnerDiskGb > 0)
	VmRunnerNetworkQuotaBytes   uint64        // Total network bytes (rx+tx) allowed per VM job before it is killed (0 = unlimited)
	DisableOutputTTLCleanup     bool          // Doesn't auto start cleaning up TTL'd logs
	OutputTTL                   time.Duration // TTL of the job output logs
	OutputTTLCleanupInterval    time.Duration // How often to check for TTL'd logs
	LogDiskUsageComputeInterval time.Duration // How often to measure how much disk is used for the output logs
	MaxLogDiskUsage             int64         // Max disk (bytes) available for storing all logs (not that strongly enforced)
}

func NewConfig(
	maxParallelJobs int32,
	maxOutputSize int64,
	runnerMemoryBytes int64,
	containerRunnerNanoCpu int64,
	vmRunnerCpu uint,
	vmRunnerNetworkMbps uint64,
	vmRunnerDiskGb uint,
	vmRunnerDiskPool string,
	vmRunnerDiskReadIops uint,
	vmRunnerDiskWriteIops uint,
	vmRunnerNetworkQuotaBytes uint64,
	disableOutputTTLCleanup bool,
	outputTTL time.Duration,
	outputTTLCleanupInterval time.Duration,
	logDiskUsageComputeInterval time.Duration,
	maxLogDiskUsage int64,
) Config {
	return Config{
		MaxParallelJobs:             maxParallelJobs,
		MaxOutputSize:               maxOutputSize,
		RunnerMemoryBytes:           runnerMemoryBytes,
		ContainerRunnerNanoCpu:      containerRunnerNanoCpu,
		VmRunnerCpu:                 vmRunnerCpu,
		VmRunnerNetworkMbps:         vmRunnerNetworkMbps,
		VmRunnerDiskGb:              vmRunnerDiskGb,
		VmRunnerDiskPool:            vmRunnerDiskPool,
		VmRunnerDiskReadIops:        vmRunnerDiskReadIops,
		VmRunnerDiskWriteIops:       vmRunnerDiskWriteIops,
		VmRunnerNetworkQuotaBytes:   vmRunnerNetworkQuotaBytes,
		DisableOutputTTLCleanup:     disableOutputTTLCleanup,
		OutputTTL:                   outputTTL,
		OutputTTLCleanupInterval:    outputTTLCleanupInterval,
		LogDiskUsageComputeInterval: logDiskUsageComputeInterval,
		MaxLogDiskUsage:             maxLogDiskUsage,
	}
}

// Returns the default suggested config
func NewDefaultConfig() Config {
	return NewConfig(
		/*maxParallelJobs*/ 5,
		/*maxOutputSize*/ 5*1024*1024, // 5MB
		/*runnerMemoryBytes*/ 1*1024*1024*1024,
		/*containerRunnerNanoCpu*/ 800_000_000, // 0.8 CPU
		/*vmRunnerCpu*/ 1,
		/*vmRunnerNetworkMbps*/ 100,
		/*vmRunnerDiskGb*/ 15,
		/*vmRunnerDiskPool*/ "default",
		/*vmRunnerDiskReadIops*/ 10000,
		/*vmRunnerDiskWriteIops*/ 5000,
		/*vmRunnerNetworkQuotaBytes*/ 5*1024*1024*1024, // 5GB
		/*disableOutputTTLCleanup*/ false,
		/*outputTTL*/ 7*24*time.Hour,
		/*outputTTLCleanupInterval*/ 12*time.Hour,
		/*logDiskUsageComputeInterval*/ 10*time.Minute,
		/*maxLogDiskUsage*/ 5*1024*1024*1024, // 5 GB
	)
}

// CheckCanRun is a helper to determine which jobs can be run in the current
// environment.
func CheckCanRun() (canRunDockerJobs bool, canRunVmJobs bool, err error) {
	canRunDockerJobs, err = checkDockerEnv()
	if err != nil {
		return
	}
	canRunVmJobs, err = checkLxdEnv()
	return
}

const (
	StdOutPrefix = "" // Prefix that identifies StdOut lines in the combined output
	StdErrPrefix = "" // Prefix that identifies StdErr lines in the combined output
)

var (
	ErrTooManyJobs        = errors.New("too many jobs running in parallel - try later")
	ErrOutputDiskIsFull   = errors.New("output logs disk is full")
	ErrDockerNotAvailable = errors.New("docker is not available in this environment")
	ErrVmNotAvailable     = errors.New("lxd is not available in this environment")
)
