//go:build linux
// +build linux

package runners

import (
	"bytes"
	"errors"
	"io"
	"log"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Set in TestMain; used by individual tests to skip if a runtime is not available.
var (
	testCanRunDockerJobs bool
	testCanRunVmJobs     bool
)

func TestMain(m *testing.M) {
	// hint: `sudo usermod -aG lxd $USER` might be required for lxd
	var err error
	testCanRunDockerJobs, testCanRunVmJobs, err = CheckCanRun()
	if err != nil {
		log.Printf("CheckCanRun: %s", err)
		return
	}
	os.Exit(m.Run())
}

func TestConstructorFailsIfCanRunErrors(t *testing.T) {
	mockCanRun := func() (canRunDockerJobs bool, canRunVmJobs bool, err error) {
		err = errors.New("BOOOOMMM")
		return
	}
	cfg := NewDefaultConfig()
	_, err := NewService(t.TempDir(),
		/*requiresGVisor*/ false,
		/*dontUseGVisor*/ true,
		/*autoCleanupDockerAndVms*/ false,
		cfg, mockCanRun)
	if err == nil || !strings.Contains(err.Error(), "BOOOOMMM") {
		t.Fatalf("err=%s, expected to contain BOOOOMMM", err)
	}
}

func TestReturnsErrorsIfCantRunDockerNorVm(t *testing.T) {
	mockCanRun := func() (canRunDockerJobs bool, canRunVmJobs bool, err error) {
		canRunDockerJobs = false
		canRunVmJobs = false
		err = nil
		return
	}
	cfg := NewDefaultConfig()
	s, err := NewService(t.TempDir(),
		/*requiresGVisor*/ false,
		/*dontUseGVisor*/ true,
		/*autoCleanupDockerAndVms*/ false,
		cfg, mockCanRun)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	canRunDocker, canRunVm := s.GetCanRun()
	if canRunDocker {
		t.Fatalf("got canRunDocker when mock says false")
	}
	if canRunVm {
		t.Fatalf("got canRunVm when mock says false")
	}

	// Try running in docker -> expect err bc we mocked docker not available
	_, err = s.Run("say-hi-in-docker", runnerlib.JobPayload{
		Name:      "say hi",
		ImageName: runnerlib.BaseImage,
		Steps: []runnerlib.JobStep{
			{Run: "echo hi"},
		},
		TimeoutMilliSeconds: 2000,
	})
	if !errors.Is(err, ErrDockerNotAvailable) {
		t.Fatalf("expected ErrDockerNotAvailable, got=%s", err)
	}

	// Try running in vm -> expect err bc we mocked docker not available
	_, err = s.Run("say-hi-in-vm", runnerlib.JobPayload{
		Name:      "say hi",
		ImageName: runnerlib.VmImage,
		Steps: []runnerlib.JobStep{
			{Run: "echo hi"},
		},
		TimeoutMilliSeconds: 2000,
	})
	if !errors.Is(err, ErrVmNotAvailable) {
		t.Fatalf("expected ErrVmNotAvailable, got=%s", err)
	}
}

func TestSimpleCases(t *testing.T) {
	if !testCanRunDockerJobs {
		t.Skip()
	}
	// Since setting up a test is relativelly costly, we should try to reuse
	// this test for many test-cases
	s := setupTest(t)

	// Simple `echo hi`
	st1, err := s.Run("say-hi", runnerlib.JobPayload{
		Name: "say hi",
		Steps: []runnerlib.JobStep{
			{Run: "echo hi"},
		},
		TimeoutMilliSeconds: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st1 != trackclient.TrackJobStatusSuccess {
		t.Fatalf("say-hi unexpected status: %q", st1)
	}
	checkJobOutputContains(s, "say-hi", "hi", t)

	// Simple job that necessarily fails
	st2, err := s.Run("fail", runnerlib.JobPayload{
		Name: "fail",
		Steps: []runnerlib.JobStep{
			{Run: "false"},
		},
		TimeoutMilliSeconds: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st2 != trackclient.TrackJobStatusFail {
		t.Fatalf("fail unexpected status: %q", st2)
	}

	// Simple job that necessarily timeouts
	st3, err := s.Run("timeout", runnerlib.JobPayload{
		Name: "fail",
		Steps: []runnerlib.JobStep{
			{Run: "sleep 100000"},
		},
		TimeoutMilliSeconds: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st3 != trackclient.TrackJobStatusTimeout {
		t.Fatalf("timeout unexpected status: %q", st3)
	}
}

func TestMaxParallelRunners(t *testing.T) {
	if !testCanRunDockerJobs {
		t.Skip()
	}
	with1JobLimit := func(c *Config) {
		c.MaxParallelJobs = 1
	}
	s := setupTest(t, with1JobLimit)
	run1ErrCh := make(chan error, 1)
	go func() {
		_, err := s.Run("sleep 200 ms", runnerlib.JobPayload{
			Name: "sleep",
			Steps: []runnerlib.JobStep{
				{Run: "sleep 0.2"},
			},
			TimeoutMilliSeconds: 1000,
		})
		run1ErrCh <- err
	}()
	for s.JobsRunning() != 1 {
		time.Sleep(10 * time.Millisecond)
	}
	_, err := s.Run("try-but-fail", runnerlib.JobPayload{})
	if !errors.Is(err, ErrTooManyJobs) {
		t.Fatalf("expected too many runners err, got %s", err)
	}
	err = <-run1ErrCh
	if err != nil {
		t.Fatal(err)
	}
}

func TestOutputTTL(t *testing.T) {
	if !testCanRunDockerJobs {
		t.Skip()
	}
	// Change the config so that logs are deleted as soon as cleanup starts
	withTTL := func(c *Config) {
		c.DisableOutputTTLCleanup = true
		c.OutputTTL = 1 * time.Millisecond
		c.OutputTTLCleanupInterval = 50 * time.Millisecond
	}
	s := setupTest(t, withTTL)

	// Run a simple job that logs "hi"
	st1, err := s.Run("say-hi", runnerlib.JobPayload{
		Name: "say hi",
		Steps: []runnerlib.JobStep{
			{Run: "echo hi"},
		},
		TimeoutMilliSeconds: 2000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st1 != trackclient.TrackJobStatusSuccess {
		t.Fatalf("say-hi unexpected status: %q", st1)
	}
	checkJobOutputContains(s, "say-hi", "hi", t)
	s.EnableOutputTTLCleanup()

	// Wait for the first cleanup to happen
	start := time.Now()
	for s.OutputTTLCleanupCount() < 1 {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			t.Fatal("spent too long waiting for TTL")
		}
	}
	_, isNotFoundErr, _ := s.Read("say-hi")
	if !isNotFoundErr {
		t.Fatalf("log was not TTL'd")
	}

	// Ensure TTL keeps on happening
	start = time.Now()
	for s.OutputTTLCleanupCount() < 5 {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			t.Fatal("spent too long waiting for TTL")
		}
	}
}

func TestOutputDiskLimit(t *testing.T) {
	if !testCanRunDockerJobs {
		t.Skip()
	}
	// Change the config so that logs are deleted as soon as cleanup starts
	withCustomTtlAndDiskSize := func(c *Config) {
		c.DisableOutputTTLCleanup = true
		c.OutputTTL = 1 * time.Millisecond
		c.OutputTTLCleanupInterval = 10 * time.Millisecond
		c.LogDiskUsageComputeInterval = 50 * time.Millisecond
		c.MaxLogDiskUsage = 1 // Can only use 1 Byte - this highlights that this is not that strongly enforced
	}
	s := setupTest(t, withCustomTtlAndDiskSize)

	if s.OutputLogsDiskUsageComputationCount() < 1 {
		t.Fatal("disk usage should be computed on creation")
	}
	if s.OutputLogsDiskUsage() > 0 {
		t.Fatalf("bad disk usage: %d", s.OutputLogsDiskUsage())
	}
	// Run a simple job that dumps some bytes to the output
	_, err := s.Run("job-id", runnerlib.JobPayload{
		Name: "dump 300 to output",
		Steps: []runnerlib.JobStep{
			{Run: "head -c 300 /dev/zero"},
		},
		TimeoutMilliSeconds: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Wait for the disk usage to be computed
	initialCount := s.OutputLogsDiskUsageComputationCount()
	start := time.Now()
	for s.OutputLogsDiskUsageComputationCount() == initialCount {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			t.Fatal("spent too long waiting for disk usage computation")
		}
	}
	if s.OutputLogsDiskUsage() == 0 || s.OutputLogsDiskUsage() < 300 {
		t.Fatalf("bad disk usage: %d", s.OutputLogsDiskUsage())
	}
	// Since the disk is now full, we cant start another job
	_, err = s.Run("job-id-2", runnerlib.JobPayload{})
	if !errors.Is(err, ErrOutputDiskIsFull) {
		t.Fatalf("didn't get output is full: %s", err)
	}
	// Enable the cleanup and wait for one cleanup to happen
	s.EnableOutputTTLCleanup()
	start = time.Now()
	for s.OutputTTLCleanupCount() < 1 {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			t.Fatal("spent too long waiting for TTL")
		}
	}
	// Wait for the disk usage to be recomputed
	initialCount = s.OutputLogsDiskUsageComputationCount()
	start = time.Now()
	for s.OutputLogsDiskUsageComputationCount() == initialCount {
		time.Sleep(10 * time.Millisecond)
		if time.Since(start) > 5*time.Second {
			t.Fatal("spent too long waiting for disk usage computation")
		}
	}
	if s.OutputLogsDiskUsage() > 0 {
		t.Fatalf("bad disk usage: %d", s.OutputLogsDiskUsage())
	}
}

func TestCantRunTwoJobsWithSameIdAndCancelation(t *testing.T) {
	if !testCanRunDockerJobs {
		t.Skip()
	}
	s := setupTest(t)

	// Start a routine that will run a job that sleeps forever
	var st trackclient.TrackJobStatus
	var runErr error
	var wg sync.WaitGroup
	wg.Add(1)
	const jobId = "sleep-for-60s-job-id"
	go func() {
		st, runErr = s.Run(jobId, runnerlib.JobPayload{
			Name: "sleep for 60s",
			Steps: []runnerlib.JobStep{
				{Run: "sleep 60"},
			},
			TimeoutMilliSeconds: 120_000,
		})
		wg.Done()
	}()
	// Wait for the job to start
	for runErr == nil && s.JobsRunning() == 0 {
		time.Sleep(time.Second)
	}
	if runErr != nil {
		t.Fatal(runErr)
	}

	// Try starting a job with the same job Id. Should get an error.
	_, runSameJobIdErr := s.Run(jobId, runnerlib.JobPayload{
		Name: "sleep for 60s #2",
		Steps: []runnerlib.JobStep{
			{Run: "sleep 60"},
		},
		TimeoutMilliSeconds: 120_000,
	})
	if runSameJobIdErr == nil {
		t.Fatal("got no err trying to run same jobId")
	}

	// Cancel the job
	s.Cancel(jobId)
	s.Cancel(jobId) // Should be idempotent
	wg.Wait()
	if runErr != nil {
		t.Fatal(runErr)
	}
	if st != trackclient.TrackJobStatusCancel {
		t.Fatalf("unexpected staus: %s", st)
	}

	s.Cancel(jobId) // Canceling again should do nothing

	out, _, err := s.Read(jobId)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	buff := bytes.NewBuffer(nil)
	io.Copy(buff, out)
	outStr := buff.String()
	if !strings.Contains(outStr, "CANCEL") {
		t.Fatalf("output doesnt say CANCEL: %s", outStr)
	}
}

func TestVmManyCases(t *testing.T) {
	if !testCanRunVmJobs {
		t.Skip()
	}
	// Run many tests with the same instance
	// because initializing the service is relativelly costly
	s := setupTest(t)

	// Happy path of a simple job
	{
		st, err := s.Run("vm-echo-hi", runnerlib.JobPayload{
			Name:                "say hi from vm",
			ImageName:           runnerlib.VmImage,
			Steps:               []runnerlib.JobStep{{Run: "echo hi from vm"}},
			TimeoutMilliSeconds: 1_000,
		})
		if err != nil {
			t.Fatal(err)
		}
		if st != trackclient.TrackJobStatusSuccess {
			t.Fatalf("unexpected status: %q", st)
		}
		checkJobOutputContains(s, "vm-echo-hi", "hi from vm", t)
	}

	// Job cancelation
	{
		runIsDoneCh := make(chan bool, 1)
		var st trackclient.TrackJobStatus
		var err error
		go func() {
			st, err = s.Run("vm-sleep-forever", runnerlib.JobPayload{
				Name:                "sleep forever",
				ImageName:           runnerlib.VmImage,
				Steps:               []runnerlib.JobStep{{Run: "sleep 1000000"}},
				TimeoutMilliSeconds: 1_000_000_000,
			})
			runIsDoneCh <- true
		}()
		for err == nil && s.JobsRunning() == 0 {
			time.Sleep(time.Second)
		}
		if err != nil {
			t.Fatal(err)
		}
		s.Cancel("vm-sleep-forever")
		<-runIsDoneCh
		if err != nil {
			t.Fatal(err)
		}
		if st != trackclient.TrackJobStatusCancel {
			t.Fatalf("bad vm status after cancel: %s", st)
		}
		checkJobOutputContains(s, "vm-sleep-forever", "CANCELED", t)
	}

	// Timeout
	{
		st, err := s.Run("vm-sleep-until-timeout", runnerlib.JobPayload{
			Name:                "sleep until timeout",
			ImageName:           runnerlib.VmImage,
			Steps:               []runnerlib.JobStep{{Run: "sleep 1000000"}},
			TimeoutMilliSeconds: 100,
		})
		if err != nil {
			t.Fatal(err)
		}
		if st != trackclient.TrackJobStatusTimeout {
			t.Fatalf("bad vm status after timeout: %s", st)
		}
		checkJobOutputContains(s, "vm-sleep-until-timeout", "TIMED OUT", t)
	}

}

func TestVmNetworkQuota(t *testing.T) {
	if !testCanRunVmJobs {
		t.Skip()
	}
	// Set a 1-byte quota so any outbound request triggers the kill.
	withTinyQuota := func(c *Config) {
		c.VmRunnerNetworkQuotaBytes = 1
	}
	s := setupTest(t, withTinyQuota)

	st, err := s.Run("vm-net-quota", runnerlib.JobPayload{
		Name:      "trigger network quota",
		ImageName: runnerlib.VmImage,
		Steps: []runnerlib.JobStep{
			// Generate traffic; the semicolon ensures sleep runs even if curl fails,
			// so the quota poller (2s interval) always has time to fire.
			{Run: "curl -s https://twigg.vc -o /dev/null"},
			{Run: "sleep 30"},
		},
		TimeoutMilliSeconds: 60_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st != trackclient.TrackJobStatusFail {
		t.Fatalf("expected fail status for network quota, got %q", st)
	}
	checkJobOutputContains(s, "vm-net-quota", "EXCEEDED NETWORK QUOTA", t)
}

func setupTest(t *testing.T, options ...func(*Config)) Service {
	t.Helper()
	cfg := NewDefaultConfig()
	for _, option := range options {
		option(&cfg)
	}
	// Since tests run in parallel when they're in different packages,
	// tests must run with autoCleanupDocker=false
	s, err := NewService(t.TempDir() /*requiresGVisor*/, false,
		/*dontUseGVisor*/ true /*autoCleanupDocker*/, false, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func checkJobOutputContains(s Service, jobId string, expectedOutToContain string, t *testing.T) {
	t.Helper()
	out, isNotFoundErr, err := s.Read(jobId)
	if isNotFoundErr {
		t.Fatalf("got notFoundErr for jobId=%s", jobId)
	}
	if err != nil {
		t.Fatalf("got err getting out of jobId=%s: %s", jobId, err)
	}
	defer out.Close()
	buff := bytes.NewBuffer(nil)
	_, err = io.Copy(buff, out)
	if err != nil {
		t.Fatalf("got err reading out of jobId=%s: %s", jobId, err)
	}
	gotOut := buff.String()
	if !strings.Contains(gotOut, expectedOutToContain) {
		t.Fatalf("unexpected output of jobId=%s. Expected to contain %q got %q", jobId, expectedOutToContain, gotOut)
	}
}