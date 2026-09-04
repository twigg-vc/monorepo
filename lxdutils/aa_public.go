// This package contains some helpers for LXD commands
package lxdutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

// ForceStops an instance
func ForceStop(srv lxd.InstanceServer, instanceName string, timeout time.Duration) error {
	ctx, cancelCtx := context.WithTimeout(context.Background(), timeout)
	defer cancelCtx()
	op, err := srv.UpdateInstanceState(instanceName, api.InstanceStatePut{
		Action:  "stop",
		Timeout: 0,
		Force:   true,
	}, "")
	if err != nil {
		return err
	}
	return WaitOpOrTimeout(op, ctx, "ForceStop timed out")
}

// Stops an instance if its still running and deletes it
func ForceDelete(srv lxd.InstanceServer, instanceName string, stopTimeout, deleteTimeout time.Duration) error {
	_ = ForceStop(srv, instanceName, stopTimeout) // Probably not needed, but doesnt hurt
	ctx, cancelCtx := context.WithTimeout(context.Background(), deleteTimeout)
	defer cancelCtx()
	op, err := srv.DeleteInstance(instanceName, true)
	if err != nil {
		return err
	}
	return WaitOpOrTimeout(op, ctx, "ForceDelete timed out")
}

// Waits until the VM Agent is responsive inside an instance.
// Polls with a no-op exec until the guest agent inside the VM is reachable.
func WaitForAgent(srv lxd.InstanceServer, instanceName string, ctx context.Context) error {
	const timeoutErrMsg = "timed out waiting for LXD VM agent"
	for {
		op, err := srv.ExecInstance(instanceName, api.InstanceExecPost{
			Command:     []string{"true"},
			WaitForWS:   true,
			Interactive: false,
		}, &lxd.InstanceExecArgs{
			Stdin:    bytes.NewReader(nil),
			DataDone: make(chan bool, 1),
		})
		if err == nil {
			opWaitErr := WaitOpOrTimeout(op, ctx, "")
			if opWaitErr == nil {
				// agent accepted the exec and `true` ran
				// -> VM is definitively ready
				return nil
			}
		}
		// Check for timeout; else the for loop runs forever if ExecInstance errors
		select {
		case <-ctx.Done():
			return errors.New(timeoutErrMsg)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Waits until "cloud-init status --wait" returns 0.
func WaitForCloudInit(srv lxd.InstanceServer, name string, ctx context.Context) error {
	const timeoutErrMsg = "timed out waiting for cloud-init"
	for {
		op, err := srv.ExecInstance(name, api.InstanceExecPost{
			Command:     []string{"cloud-init", "status", "--wait"},
			WaitForWS:   true,
			Interactive: false,
		}, &lxd.InstanceExecArgs{
			Stdin:    bytes.NewReader(nil),
			DataDone: make(chan bool, 1),
		})
		if err == nil {
			opWaitErr := WaitOpOrTimeout(op, ctx, "")
			if opWaitErr == nil {
				metadata := op.Get().Metadata
				if retCode, ok := metadata["return"].(float64); ok && retCode == 0 {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return errors.New(timeoutErrMsg)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Waits until the network is responsive inside an instance.
// First polls GetInstanceState until at least one non-loopback interface has a
// globally-scoped IPv4 address (i.e. DHCP completed), then verifies actual
// external connectivity by pinging a reliable IP (e.g. 8.8.8.8)
// from inside the instance.
func WaitForNetwork(srv lxd.InstanceServer, instanceName string, ctx context.Context) error {
	var lastPingErr error
	for {
		state, _, err := srv.GetInstanceState(instanceName)
		if err == nil {
			for name, iface := range state.Network {
				if name == "lo" {
					continue
				}
				for _, addr := range iface.Addresses {
					if addr.Family == "inet" && addr.Scope == "global" {
						if pingErr := pingExternal(srv, instanceName, ctx); pingErr == nil {
							return nil
						} else {
							lastPingErr = pingErr
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			if lastPingErr != nil {
				return fmt.Errorf("timed out waiting for VM %q external network; the VM has an IP but can't reach the internet, so forwarding from its bridge is likely blocked by another firewall table — if docker runs on this host, its iptables FORWARD policy is DROP and the bridge needs accepts in the DOCKER-USER chain (see twigg-track-docker-user.service). Last ping error: %w", instanceName, lastPingErr)
			}
			return fmt.Errorf("timed out waiting for VM %q to acquire IPv4 address", instanceName)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func pingExternal(srv lxd.InstanceServer, instanceName string, ctx context.Context) error {
	dataDone := make(chan bool, 1)
	const reliablePublicIp = "8.8.8.8"
	op, err := srv.ExecInstance(instanceName, api.InstanceExecPost{
		Command: []string{"ping",
			"-c1", // send only 1 packet
			"-W5", // wait up to 5s before giginv up
			reliablePublicIp},
		WaitForWS:   true,
		Interactive: false,
	}, &lxd.InstanceExecArgs{
		Stdin:    bytes.NewReader(nil),
		DataDone: dataDone,
	})
	if err != nil {
		return err
	}
	if err := WaitOpOrTimeout(op, ctx, ""); err != nil {
		return err
	}
	<-dataDone
	if retCode, ok := op.Get().Metadata["return"].(float64); ok && retCode != 0 {
		return errors.New("ping failed")
	}
	return nil
}

func WaitOpOrTimeout(op lxd.Operation, timeoutCtx context.Context, timeoutErrMsg string) error {
	errCh := make(chan error, 1)
	go func() { errCh <- op.Wait() }()
	select {
	case <-timeoutCtx.Done():

		// Cancel and try to drain the errCh
		_ = op.Cancel()
		select {
		case <-errCh:
		case <-time.After(5 * time.Second): // But dont block forever
			// If we get here its probably ok, errCh will eventually drain if
			// the LXD lib is properly implemetned
		}

		if timeoutErrMsg == "" {
			return timeoutCtx.Err()
		}
		return errors.New(timeoutErrMsg)
	case err := <-errCh:
		return err
	}
}
