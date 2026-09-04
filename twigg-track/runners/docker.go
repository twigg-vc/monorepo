package runners

import (
	"context"
	"errors"
	"fmt"
	"io"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
)

// This is used to identify the docker networks and must match the "tw-*"
// interface patterns in the nftable rules (twigg-track.nft)
// that add ACLs to the docker networks to prevent the containers from doing
// bad stuff like talking to cloud metadata endpoints.
// If changed, twigg-track.nft must be updated too.
const bridgeNamePrefix = "tw-"

func (s *service) runJobInDocker(jobId_ string, jobIdHash string, path string, j runnerlib.JobPayload,
	payload []byte, outWriter, errWriter io.Writer, manualCancelCtx context.Context) (st trackclient.TrackJobStatus, err error) {
	if !s.canRunDockerJobs {
		panic("runJobInDocker called with !s.canRunDockerJobs")
	}
	// Create a resolv.conf (DNS configuration file) that will be mounted
	// into the container. This is required to properly configure the
	// container's network: By default, docker will set this file to `127.0.0.10`
	// - which is an internal DNS server. When gVisor is used, that DNS is
	// not reachable. The solution is to manually overwrite that file to use
	// public DNSs only (Cloudflare and Google).
	resolveConfFilePath := filepath.Join(path, resolvConfFileName)
	err = os.WriteFile(resolveConfFilePath,
		[]byte("nameserver 8.8.8.8\nnameserver 1.1.1.1\n"), 0644)
	if err != nil {
		err = fmt.Errorf("failed to create resolv.conf file: %w", err)
		return
	}
	// Create the container config
	cfg := &container.Config{
		Image:        resolveImage(j),
		OpenStdin:    true,
		StdinOnce:    true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Labels: map[string]string{
			dockerLabelKey: dockerLabelVal,
		},
	}

	pidLimit := int64(512)
	netName := fmt.Sprintf("runners-net-%s", jobIdHash[:32])
	hostCfg := container.HostConfig{
		Resources: container.Resources{
			Memory:    s.cfg.RunnerMemoryBytes,
			NanoCPUs:  s.cfg.ContainerRunnerNanoCpu,
			PidsLimit: &pidLimit,
		},
		// Limit the writable layer
		StorageOpt: map[string]string{
			"size": "1G", // 1GB
		},
		NetworkMode: container.NetworkMode(netName),
		Binds: []string{fmt.Sprintf("%s:/etc/resolv.conf:ro",
			resolveConfFilePath)},
	}
	if s.shouldUseGVisor {
		hostCfg.Runtime = "runsc"
	}
	// bridgeName has max 15 characters
	bridgeName := fmt.Sprintf("%s%s", bridgeNamePrefix, jobIdHash)
	bridgeName = bridgeName[:15]
	// Create the container network using context.Background() to prevent cancelation
	_, err = s.docker.NetworkCreate(context.Background(), netName, network.CreateOptions{
		Driver:   "bridge",
		Internal: false, // allow external routing
		Labels: map[string]string{
			dockerLabelKey: dockerLabelVal,
		},
		Options: map[string]string{
			"com.docker.network.bridge.name": bridgeName,
		},
	})
	if err != nil {
		err = fmt.Errorf("runner failed to create network: %s", err)
		return
	}
	defer s.tryCleaningUpDockerNetwork(netName)

	netCfg := network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			netName: {},
		},
	}
	// Make sure network is up before creating the container
	err = waitUntilNetworkIsUp(s.docker, netName)
	if err != nil {
		return
	}

	// Create container using context.Background() to prevent cancelation
	resp, err := s.docker.ContainerCreate(context.Background(), cfg, &hostCfg, &netCfg, nil, "")
	if err != nil {
		err = fmt.Errorf("runner failed to create container: %s", err)
		return
	}
	containerID := resp.ID
	defer s.tryCleaningUpContainer(containerID)

	// Attach to container using context.Background() to prevent cancelation
	attachResp, err := s.docker.ContainerAttach(context.Background(), containerID, container.AttachOptions{
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Stream: true,
	})
	if err != nil {
		err = fmt.Errorf("runner failed to attach: %s", err)
		return
	}
	defer attachResp.Close()

	// Start container using context.Background() to prevent cancelation
	err = s.docker.ContainerStart(context.Background(), containerID, container.StartOptions{})
	if err != nil {
		err = fmt.Errorf("runner failed to start container: %s", err)
		return
	}

	// Waitgroup used to ensure all channels run to completion
	var wg sync.WaitGroup
	// Use a channel to track stdin errors and send JSON payload to it
	stdinErrCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer attachResp.CloseWrite()
		_, stdinWriteErr := attachResp.Conn.Write(payload)
		stdinErrCh <- stdinWriteErr
	}()
	// Stream logs (this blocks until container stops or connection closes)
	// We run this in a goroutine so we can select on the context
	streamLogsErrCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, copyErr := stdcopy.StdCopy(outWriter, errWriter, attachResp.Reader)
		streamLogsErrCh <- copyErr
	}()

	// Create a simple waitCtx that is never canceled unless
	// cancelWaitCtx is called. This is passed to the ContainerWait
	waitCtx, cancelWaitCtx := context.WithCancel(context.Background())
	defer cancelWaitCtx()
	containerWaitStatusCh, containerWaitErrCh := s.docker.ContainerWait(waitCtx,
		containerID, container.WaitConditionNotRunning)
	var waitErr error
	var waitResp container.WaitResponse

	// Create a timer to cancel the wait
	timeoutTimer := time.NewTimer(time.Duration(j.TimeoutMilliSeconds) * time.Millisecond)
	defer timeoutTimer.Stop()
	isTimeoutExceeded := false

	// Create a manual cancelation will use the manualCancelCtx created at
	// the top of the method
	isManualCancel := false

	// Helper function to to the necessary cleanup after a timeout or manual
	// cancelation is done
	cleanupAfterManualOrTimeoutCancel := func() {
		// Cancel the ContainerWait bc it wont be used anymore
		cancelWaitCtx()
		// Immediatelly kill container
		_ = s.docker.ContainerKill(context.Background(), containerID, "SIGKILL")
		// Close the attach resp to prevent StdCopy from hanging
		attachResp.Close()
	}

	// Select one of the stop conditions
	start := time.Now()
	select {
	case <-timeoutTimer.C:
		cleanupAfterManualOrTimeoutCancel()
		isTimeoutExceeded = true
	case <-manualCancelCtx.Done():
		cleanupAfterManualOrTimeoutCancel()
		isManualCancel = true
	case errChErr := <-containerWaitErrCh:
		waitErr = fmt.Errorf("container wait error: %w", errChErr)
		// Close the attach resp to prevent StdCopy from hanging
		attachResp.Close()
		// Don't return immediatelly. Wait for the waitgroup.
	case statusResp := <-containerWaitStatusCh:
		// Container finished naturally
		waitErr = nil
		waitResp = statusResp
	}
	// Wait for goroutines to finish their last writes before we trigger file defers
	wg.Wait()

	// Check if we got an error from the select above:
	if waitErr != nil {
		err = fmt.Errorf("runner failed to wait: %s", waitErr)
		return
	}
	// Finish waiting for other chanels
	stdInErr := <-stdinErrCh
	if stdInErr != nil {
		err = fmt.Errorf("runner failed to pipe stdin: %w", stdInErr)
		return
	}
	streamLogsErr := <-streamLogsErrCh
	// We expect an error when streaming the logs if we forcefully closed
	// the container due to timeout, so we only care for unexpected errors
	if streamLogsErr != nil &&
		!errors.Is(streamLogsErr, net.ErrClosed) {
		err = fmt.Errorf("runner failed to copy logs: %w", streamLogsErr)
		return
	}

	isSuccess := !isTimeoutExceeded && !isManualCancel && waitResp.StatusCode == 0
	isOOM := false
	isRuntimeErr := false
	isSigkill := false
	failureExitCode := waitResp.StatusCode
	if !isSuccess && !isTimeoutExceeded && !isManualCancel {
		inspect, err := s.docker.ContainerInspect(context.Background(), containerID)
		if err != nil {
			return "", fmt.Errorf("runner failed to inspect: %w", err)
		}
		isOOM = inspect.State.OOMKilled
		isRuntimeErr = inspect.State.Error != ""
		isSigkill = !isOOM && inspect.State.ExitCode == 137
	}
	const isNetworkQuotaExceeded = false
	st, err = logOutput(start,
		isTimeoutExceeded, isManualCancel, isNetworkQuotaExceeded, isSuccess, isOOM,
		isSigkill, isRuntimeErr, failureExitCode, outWriter)
	if err != nil {
		return
	}
	return
}

// Tries for a few times to cleanup a network. Logs error if not succesfull.
func (s *service) tryCleaningUpDockerNetwork(netName string) {
	cleanupFunc := func() error {
		return removeNetwork(s.docker, netName)
	}
	runCleanup(fmt.Sprintf("remove network %s", netName), cleanupFunc)
}

// Tries for a few times to killAndRemove a container. Logs error if not successfull.
func (s *service) tryCleaningUpContainer(containerId string) {
	cleanupFunc := func() error {
		return killAndRemoveContainer(s.docker, containerId)
	}
	runCleanup(fmt.Sprintf("remove container %s", containerId), cleanupFunc)
}

const (
	dockerBaseRunnerImage   = "twigg-base-runner:latest"
	dockerGoRunnerImage     = "twigg-go-runner:latest"
	dockerNode20RunnerImage = "twigg-node-20-runner:latest"
	dockerBunRunnerImage    = "twigg-bun-runner:latest"
)

// Returns the name of the image the job must run under.
// When nothing matches, returns the base image
func resolveImage(j runnerlib.JobPayload) string {
	switch j.ImageName {
	case runnerlib.GoImage:
		return dockerGoRunnerImage
	case runnerlib.Node20Image:
		return dockerNode20RunnerImage
	case runnerlib.BunImage:
		return dockerBunRunnerImage
	default:
		return dockerBaseRunnerImage
	}
}

// Run a function that panics if we change the bridgeName just so we're reminded
// that we cant do it without also change the nftables
var _ = func() bool {
	if bridgeNamePrefix != "tw-" {
		panic("bridgeNamePrefix changed! Did you remember to change the nftable rules in twigg-track.nft?")
	}
	if len(bridgeNamePrefix) >= 10 {
		panic("bridgeNamePrefix >=10. A small lenght must be used bc the name has a 15 len limit")
	}
	return true
}()
