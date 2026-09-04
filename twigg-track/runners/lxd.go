package runners

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"monorepo/lxdutils"
	"monorepo/twigg-runner/runnerlib"
	"monorepo/twigg-track/trackclient"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

const lxdNetworkQuotaPollInterval = 2 * time.Second

func (s *service) runJobInLxd(jobId_ string, jobIdHash string, path_ string, j runnerlib.JobPayload,
	payload []byte, outWriter, errWriter io.Writer, manualCancelCtx context.Context) (st trackclient.TrackJobStatus, err error) {
	if !s.canRunVmJobs {
		panic("runJobInLxd called with !s.canRunVmJobs")
	}

	if len(jobIdHash) < 32 {
		err = fmt.Errorf("jobIdHash length < 32")
		return
	}
	instanceName := fmt.Sprintf("twigg-job-%s", jobIdHash[:32])
	vmSetupStart := time.Now()

	// Create and start the VM
	createOp, err := s.lxdServer.CreateInstance(api.InstancesPost{
		Name: instanceName,
		Type: api.InstanceTypeVM,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: lxdVMImage,
		},
		InstancePut: api.InstancePut{
			Config: map[string]string{
				"limits.memory": fmt.Sprintf("%d", s.cfg.RunnerMemoryBytes),
				"limits.cpu":    fmt.Sprintf("%d", s.cfg.VmRunnerCpu),
				lxdLabelKey:     lxdLabelVal,
			},
			Devices: s.lxdInstanceDevices(lxdRunnerNetName),
		},
	})
	if err != nil {
		err = fmt.Errorf("runner failed to create LXD instance: %w", err)
		return
	}
	defer s.tryCleaningUpLxdInstance(instanceName)
	if err = createOp.Wait(); err != nil {
		err = fmt.Errorf("runner failed to wait for instance creation: %w", err)
		return
	}
	startOp, err := s.lxdServer.UpdateInstanceState(instanceName, api.InstanceStatePut{
		Action:  "start",
		Timeout: -1,
	}, "")
	if err != nil {
		err = fmt.Errorf("runner failed to start LXD instance: %w", err)
		return
	}
	if err = startOp.Wait(); err != nil {
		err = fmt.Errorf("runner failed to wait for instance start: %w", err)
		return
	}

	// Wait for the guest agent inside the VM to become reachable before executing.
	// This is required because we use the agent to run the binary.
	if err = waitForLxdVMAgent(s.lxdServer, instanceName); err != nil {
		err = fmt.Errorf("LXD VM agent did not become ready: %w", err)
		return
	}
	// Wait for the VM's network interface to obtain a DHCP lease before running
	// the binary. The agent becomes ready before DHCP finishes, so without this
	// wait DNS lookups might fail.
	if err = waitForLxdVMNetwork(s.lxdServer, instanceName); err != nil {
		err = fmt.Errorf("LXD VM network did not become ready: %w", err)
		return
	}

	// Write msg indicating how long the VM took to get ready
	_, err = fmt.Fprintf(outWriter, "VM SETUP TOOK %s\n\n", time.Since(vmSetupStart))
	if err != nil {
		err = fmt.Errorf("failed to write VM setup time: %w", err)
		return
	}

	// Create a clean working directory for the job so the runner
	err = mkdirInLxdInstance(s.lxdServer, instanceName, lxdJobWorkDir)
	if err != nil {
		err = fmt.Errorf("failed to create job work dir in LXD instance: %w", err)
		return
	}

	// Execute the runner binary and pass it the payload
	dataDoneChan := make(chan bool, 1)
	execOp, err := s.lxdServer.ExecInstance(instanceName, api.InstanceExecPost{
		Command:     []string{lxdRunnerBinPath},
		WaitForWS:   true,
		Interactive: false,
		Cwd:         lxdJobWorkDir,
	}, &lxd.InstanceExecArgs{
		Stdin:    bytes.NewReader(payload),
		Stdout:   outWriter,
		Stderr:   errWriter,
		DataDone: dataDoneChan,
	})
	if err != nil {
		err = fmt.Errorf("runner failed to exec in LXD instance: %w", err)
		return
	}
	execErrCh := make(chan error, 1)
	go func() {
		execErrCh <- execOp.Wait()
	}()

	timeoutTimer := time.NewTimer(time.Duration(j.TimeoutMilliSeconds) * time.Millisecond)
	defer timeoutTimer.Stop()

	networkQuotaPollCtx, cancelNetworkQuotaPollCtx := context.WithCancel(context.Background())
	defer cancelNetworkQuotaPollCtx()
	networkQuotaExceededCh := make(chan struct{}, 1)
	if s.cfg.VmRunnerNetworkQuotaBytes > 0 {
		go s.lxdPollNetworkQuota(networkQuotaPollCtx, instanceName,
			s.cfg.VmRunnerNetworkQuotaBytes, networkQuotaExceededCh)
	}

	start := time.Now()
	isNaturalExit := false
	isTimeoutExceeded := false
	isManualCancel := false
	isNetworkQuotaExceeded := false
	var execErr error
	const forceStopTimeout time.Duration = 5 * time.Second
	select {
	case <-timeoutTimer.C:
		_ = lxdutils.ForceStop(s.lxdServer, instanceName, forceStopTimeout)
		isTimeoutExceeded = true
	case <-manualCancelCtx.Done():
		_ = lxdutils.ForceStop(s.lxdServer, instanceName, forceStopTimeout)
		isManualCancel = true
	case <-networkQuotaExceededCh:
		_ = lxdutils.ForceStop(s.lxdServer, instanceName, forceStopTimeout)
		isNetworkQuotaExceeded = true
	case execErr = <-execErrCh:
		isNaturalExit = true
	}
	cancelNetworkQuotaPollCtx()
	if !isNaturalExit && !isTimeoutExceeded && !isManualCancel && !isNetworkQuotaExceeded {
		panic("Im bad at logic")
	}

	if execErr != nil {
		err = fmt.Errorf("runner exec operation failed: %w", execErr)
		return
	}

	// Try ensuring all data is writen to the output; but don't block forever
	// on edge cases that might cause it to not stop
	select {
	case <-dataDoneChan:
	case <-time.After(10 * time.Second):
		log.Printf("WARNING: dataDoneChan took a long time\n")
	}

	// Determine the exit status
	exitCode := int64(0)
	hasExitCode := false
	if isNaturalExit {
		metadata := execOp.Get().Metadata
		if retCode, ok := metadata["return"].(float64); ok {
			exitCode = int64(retCode)
			hasExitCode = true
		}
	}
	isSuccess := isNaturalExit && hasExitCode && exitCode == 0
	isOOM := isNaturalExit && hasExitCode && exitCode == 137
	isRuntimeErr := isNaturalExit && !hasExitCode
	const isSigKill = false
	st, err = logOutput(start, isTimeoutExceeded, isManualCancel, isNetworkQuotaExceeded,
		isSuccess, isOOM, isSigKill, isRuntimeErr, exitCode, outWriter)
	return
}

// waitForLxdVMAgent polls with a no-op exec until the guest agent inside the VM
// is reachable. The agent is not ready the moment the hypervisor reports "started".
func waitForLxdVMAgent(srv lxd.InstanceServer, instanceName string) error {
	timeoutCtx, cancelTimeoutCtx := context.WithTimeout(
		context.Background(), 1*time.Minute)
	defer cancelTimeoutCtx()
	return lxdutils.WaitForAgent(srv, instanceName, timeoutCtx)
}

// mkdirInLxdInstance runs "mkdir -p <path>" inside the VM via the guest agent.
func mkdirInLxdInstance(srv lxd.InstanceServer, instanceName, path string) error {
	op, err := srv.ExecInstance(instanceName, api.InstanceExecPost{
		Command:     []string{"mkdir", "-p", path},
		WaitForWS:   true,
		Interactive: false,
	}, &lxd.InstanceExecArgs{
		Stdin:    bytes.NewReader(nil),
		DataDone: make(chan bool, 1),
	})
	if err != nil {
		return err
	}
	return op.Wait()
}

// waitForLxdVMNetwork polls GetInstanceState until at least one non-loopback
// interface has a globally-scoped IPv4 address (i.e. DHCP completed).
func waitForLxdVMNetwork(srv lxd.InstanceServer, instanceName string) error {
	timeoutCtx, cancelTimeoutCtx := context.WithTimeout(context.Background(),
		1*time.Minute)
	defer cancelTimeoutCtx()
	return lxdutils.WaitForNetwork(srv, instanceName, timeoutCtx)
}

func (s *service) tryCleaningUpLxdInstance(instanceName string) {
	runCleanup(fmt.Sprintf("delete LXD instance %s", instanceName), func() error {
		return lxdutils.ForceDelete(s.lxdServer, instanceName, 15*time.Second, 15*time.Second)
	})
}

// cleanupLxdInstances deletes any VM instances that were created by this service
// but not cleaned up (e.g. after a crash). Instances are tagged with
// lxdLabelKey=lxdLabelVal on creation so they can be identified here.
func cleanupLxdInstances(srv lxd.InstanceServer) error {
	instances, err := srv.GetInstances(lxd.GetInstancesArgs{InstanceType: api.InstanceTypeVM})
	if err != nil {
		return fmt.Errorf("lxd cleanup: failed to list instances: %w", err)
	}
	for _, inst := range instances {
		if inst.Config[lxdLabelKey] != lxdLabelVal {
			continue
		}
		if err := lxdutils.ForceDelete(srv, inst.Name, 30*time.Second, 30*time.Second); err != nil {
			return fmt.Errorf("lxd cleanup: failed to delete instance %s: %w", inst.Name, err)
		}
	}
	return nil
}

func (s *service) lxdPollNetworkQuota(cancelationCtx context.Context, instanceName string,
	quotaBytes uint64, quotaExceededCh chan<- struct{}) {
	ticker := time.NewTicker(lxdNetworkQuotaPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-cancelationCtx.Done():
			return
		case <-ticker.C:
			state, _, err := s.lxdServer.GetInstanceState(instanceName)
			if err != nil {
				continue
			}
			var total uint64
			for _, net := range state.Network {
				total += net.Counters.BytesSent + net.Counters.BytesReceived
			}
			if total > quotaBytes {
				quotaExceededCh <- struct{}{}
				return
			}
		}
	}
}

func (s *service) lxdInstanceDevices(netName string) map[string]map[string]string {
	devices := map[string]map[string]string{
		"eth0": s.lxdNicDevice(netName),
	}
	if s.cfg.VmRunnerDiskGb > 0 {
		root := map[string]string{
			"type": "disk",
			"pool": s.cfg.VmRunnerDiskPool,
			"path": "/",
			"size": fmt.Sprintf("%dGB", s.cfg.VmRunnerDiskGb),
		}
		if s.cfg.VmRunnerDiskReadIops > 0 {
			root["limits.read"] = fmt.Sprintf("%diops", s.cfg.VmRunnerDiskReadIops)
		}
		if s.cfg.VmRunnerDiskWriteIops > 0 {
			root["limits.write"] = fmt.Sprintf("%diops", s.cfg.VmRunnerDiskWriteIops)
		}
		devices["root"] = root
	}

	return devices
}

func (s *service) lxdNicDevice(netName string) map[string]string {
	dev := map[string]string{
		// Use "network" (not nictype+parent) so LXD treats this as a managed network
		// attachment — required for security.ipv4_filtering to work.
		"type":    "nic",
		"network": netName,
		// Prevents ARP poisoning and IP/MAC spoofing between VMs on the shared bridge.
		// Combined with the network ACL (which already rejects RFC1918 egress), this
		// gives full L2+L3 isolation between concurrent jobs.
		"security.ipv4_filtering": "true",
	}
	if s.cfg.VmRunnerNetworkMbps > 0 {
		limit := fmt.Sprintf("%dMbit", s.cfg.VmRunnerNetworkMbps)
		dev["limits.ingress"] = limit
		dev["limits.egress"] = limit
	}
	return dev
}

// recreateLxdNetworkAndACL deletes and recreates the shared bridge network and its
// ACL from scratch on every startup, so any config changes in the binary take effect
// immediately. Network must be deleted before the ACL (it holds a reference to it).
// Callers must ensure no VM instances are attached before calling.
func recreateLxdNetworkAndACL(lxdServer lxd.InstanceServer, deleteIfExists bool) error {
	_, _, err := lxdServer.GetNetwork(lxdRunnerNetName)
	ntwkExists := err == nil
	_, _, err = lxdServer.GetNetworkACL(lxdRunnerACLName)
	aclExists := err == nil

	if deleteIfExists {
		// Delete network first (references the ACL)
		if ntwkExists {
			op, err := lxdServer.DeleteNetwork(lxdRunnerNetName)
			if err != nil {
				return fmt.Errorf("failed to delete LXD runner network: %w", err)
			}
			if err := op.Wait(); err != nil {
				return fmt.Errorf("failed to wait for LXD runner network deletion: %w", err)
			}
			ntwkExists = false
		}
		// Delete ACL
		if aclExists {
			op, err := lxdServer.DeleteNetworkACL(lxdRunnerACLName)
			if err != nil {
				return fmt.Errorf("failed to delete LXD runner ACL: %w", err)
			}
			if err := op.Wait(); err != nil {
				return fmt.Errorf("failed to wait for LXD runner ACL deletion: %w", err)
			}
			aclExists = false
		}
	}

	// Create ACL
	if !aclExists {
		aclOp, err := lxdServer.CreateNetworkACL(api.NetworkACLsPost{
			NetworkACLPost: api.NetworkACLPost{Name: lxdRunnerACLName},
			NetworkACLPut: api.NetworkACLPut{
				Description: "Restrict untrusted VMs connections",
				Egress: []api.NetworkACLRule{
					// Block private/internal destinations first — these must come before
					// any allow rules, otherwise a broad allow (e.g. DNS with no destination
					// filter) would match private IPs before the reject rules are reached.

					// Block Private Networks (RFC1918)
					{Action: "reject", Destination: "10.0.0.0/8", State: "enabled"},
					{Action: "reject", Destination: "172.16.0.0/12", State: "enabled"},
					{Action: "reject", Destination: "192.168.0.0/16", State: "enabled"},

					// Reject Cloud Metadata (Critical for security)
					{Action: "reject", Destination: "169.254.0.0/16", State: "enabled"},

					// Block the remaining non-public ranges (kept in sync with
					// the job-egress chain in twigg-track.nft)
					{Action: "reject", Destination: "0.0.0.0/8", State: "enabled"},     // "this network"
					{Action: "reject", Destination: "100.64.0.0/10", State: "enabled"}, // carrier-grade NAT (RFC6598)
					{Action: "reject", Destination: "127.0.0.0/8", State: "enabled"},   // loopback
					{Action: "reject", Destination: "224.0.0.0/4", State: "enabled"},   // multicast
					{Action: "reject", Destination: "240.0.0.0/4", State: "enabled"},   // reserved + broadcast

					// Block IPv6
					{Action: "reject", Destination: "::/0", State: "enabled"},

					// Note: we should also block to the host, but since the
					// host's IP isn't known here it must be done in the firewall.
					// In prod we use nftables input rules.

					// Allow Public Internet (including DNS — all private destinations
					// are already rejected above, so port-53 traffic can only reach public IPs)
					{Action: "allow", Destination: "0.0.0.0/0", State: "enabled"},
				},
				Ingress: []api.NetworkACLRule{
					// Default Deny for all incoming traffic to the VM
					{Action: "reject", Source: "0.0.0.0/0", State: "enabled"},
					{Action: "reject", Source: "::/0", State: "enabled"},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create LXD runner ACL: %w", err)
		}
		if err := aclOp.Wait(); err != nil {
			return fmt.Errorf("failed to wait for LXD runner ACL creation: %w", err)
		}
	}

	// Create network
	if !ntwkExists {
		netOp, err := lxdServer.CreateNetwork(api.NetworksPost{
			Name: lxdRunnerNetName,
			Type: "bridge",
			NetworkPut: api.NetworkPut{
				Config: map[string]string{
					"ipv4.address":  "auto",
					"ipv4.nat":      "true",
					"ipv6.address":  "none",
					"security.acls": lxdRunnerACLName,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create LXD runner network: %w", err)
		}
		if err := netOp.Wait(); err != nil {
			return fmt.Errorf("failed to wait for LXD runner network creation: %w", err)
		}
	}
	return nil
}

const (
	lxdVMImage       = "twigg-vm-runner"
	lxdRunnerBinPath = "/usr/local/bin/twigg-runner"

	// user.* key used to tag VM instances created by this service,
	// so they can be swept up on restart if the process crashed mid-job.
	lxdLabelKey = "user.twigg-track-runner"
	lxdLabelVal = "true"

	lxdRunnerACLName = "twigg-runner-acl"
	// lxdRunnerNetName must be ≤15 chars (Linux bridge interface name limit).
	lxdRunnerNetName = "twigg-runner-br"

	lxdJobWorkDir = "/workspace"
)

var _ = func() bool {
	if lxdRunnerNetName != "twigg-runner-br" {
		panic("the firewall rules assume the bridge is named twigg-runner-br. Did you remember to change them? (see twigg-track.nft and twigg-track-docker-user.service)")
	}
	return true
}()
