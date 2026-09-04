// Simple binary for building LXD images
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"monorepo/lxdutils"

	lxdClient "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
)

const (
	builderVmName    = "tmp-builder"
	baseImageAlias   = "twigg-ubuntu-base"
	outputImageAlias = "twigg-vm-runner"
)

var (
	pathToTwBinary          = flag.String("tw", "../../tw/bin/tw_linux_latest", "path to tw binary")
	pathToTwiggRunnerBinary = flag.String("twigg-runner", "../bin/twigg-runner-linux", "path to twigg-runner binary")
	rebuildBase             = flag.Bool("rebuild-base", false, "force rebuild of twigg-ubuntu-base even if it already exists")
)

func main() {
	flag.Parse()
	lxd, err := lxdClient.ConnectLXDUnix("", nil)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to LXD: %s", err))
	}

	if *rebuildBase || !baseImageExists(lxd) {
		fmt.Println("Building base image...")
		if ok := buildBase(lxd); !ok {
			return
		}
		fmt.Println("Done building base image")
	} else {
		fmt.Println("Base image already exists, skipping base build")
	}

	fmt.Println("Building final image...")
	if ok := buildFinal(lxd); !ok {
		return
	}
	fmt.Println("Done building final image :)")
}

func baseImageExists(lxd lxdClient.InstanceServer) bool {
	// GetImageAlias can return a random error even if the image actually
	// exists (like some IO error or something). But if it returns nil, the
	// image definitely exists.
	// Ideally we'd check the errors.Is to do a retry; but I could not find
	// decent documentation about the error types; so retrying is good enough
	const retries = 3
	for range retries {
		_, _, err := lxd.GetImageAlias(baseImageAlias)
		if err == nil {
			return true
		}
	}
	return false
}

func buildBase(lxd lxdClient.InstanceServer) (ok bool) {
	ok = cleanupVm(lxd, builderVmName /*ignoreErrors*/, true)
	if !ok {
		return
	}
	ok = cleanupImage(lxd, baseImageAlias /*ignoreErrors*/, true)
	if !ok {
		return
	}
	ok = startBuilderVm(lxd, builderVmName)
	if !ok {
		return
	}
	ok = aptUpdate(lxd, builderVmName)
	if !ok {
		return
	}
	ok = installGo(lxd, builderVmName)
	if !ok {
		return
	}
	ok = installNodeAndNpm(lxd, builderVmName)
	if !ok {
		return
	}
	ok = installBunWithNpm(lxd, builderVmName)
	if !ok {
		return
	}
	ok = installDocker(lxd, builderVmName)
	if !ok {
		return
	}
	ok = stopVmAndPublishImage(lxd, builderVmName, baseImageAlias)
	if !ok {
		return
	}
	ok = cleanupVm(lxd, builderVmName /*ignoreErrors*/, true)
	return
}

func buildFinal(lxd lxdClient.InstanceServer) (ok bool) {
	ok = cleanupVm(lxd, builderVmName /*ignoreErrors*/, true)
	if !ok {
		return
	}
	ok = cleanupImage(lxd, outputImageAlias /*ignoreErrors*/, true)
	if !ok {
		return
	}
	ok = startBuilderVmFromLocalImage(lxd, builderVmName, baseImageAlias)
	if !ok {
		return
	}
	ok = installTwiggAndTwiggRunner(lxd, builderVmName)
	if !ok {
		return
	}
	ok = resetAndShrinkVmImage(lxd, builderVmName)
	if !ok {
		return
	}
	ok = stopVmAndPublishImage(lxd, builderVmName, outputImageAlias)
	if !ok {
		return
	}
	ok = cleanupVm(lxd, builderVmName /*ignoreErrors*/, true)
	return
}

func cleanupVm(lxd lxdClient.InstanceServer, vmName string, ignoreErrors bool) (ok bool) {
	const maxTries = 3
	return Run(fmt.Sprintf("Cleanup %s VM", vmName), 15*time.Minute, maxTries, func(ctx context.Context) error {
		err := lxdutils.ForceDelete(lxd, vmName, 30*time.Second, 60*time.Second)
		if err != nil && !ignoreErrors {
			return err
		}
		return nil
	})
}

func cleanupImage(lxd lxdClient.InstanceServer, imgAlias string, ignoreErrors bool) (ok bool) {
	const maxTries = 3
	return Run(fmt.Sprintf("Cleanup %s image", imgAlias), 15*time.Minute, maxTries, func(ctx context.Context) error {
		alias, _, err := lxd.GetImageAlias(imgAlias)
		if err != nil {
			if ignoreErrors {
				return nil
			}
			return err
		}
		op, err := lxd.DeleteImage(alias.Target)
		if err != nil {
			if ignoreErrors {
				return nil
			}
			return err
		}
		return op.WaitContext(ctx)
	})
}

func startBuilderVm(lxd lxdClient.InstanceServer, vmName string) (ok bool) {
	return startVm(lxd, vmName, api.InstanceSource{
		Type:     "image",
		Server:   "https://cloud-images.ubuntu.com/releases",
		Protocol: "simplestreams",
		Alias:    "24.04",
	})
}

func startVm(lxd lxdClient.InstanceServer, vmName string, source api.InstanceSource) (ok bool) {
	const maxTries = 3
	return Run(fmt.Sprintf("Start LXD VM: %s", vmName), 15*time.Minute, maxTries, func(ctx context.Context) error {
		createOp, err := lxd.CreateInstance(api.InstancesPost{
			Name:   vmName,
			Type:   api.InstanceTypeVM,
			Source: source,
		})
		if err != nil {
			return fmt.Errorf("create instance: %w", err)
		}
		if err := createOp.WaitContext(ctx); err != nil {
			return fmt.Errorf("wait for instance creation: %w", err)
		}

		startOp, err := lxd.UpdateInstanceState(vmName, api.InstanceStatePut{
			Action:  "start",
			Timeout: -1,
		}, "")
		if err != nil {
			return fmt.Errorf("start instance: %w", err)
		}
		if err := startOp.WaitContext(ctx); err != nil {
			return fmt.Errorf("wait for instance start: %w", err)
		}

		fmt.Println("Waiting for VM to boot...")
		if err := lxdutils.WaitForAgent(lxd, vmName, ctx); err != nil {
			return fmt.Errorf("wait for agent: %w", err)
		}
		if err := lxdutils.WaitForCloudInit(lxd, vmName, ctx); err != nil {
			return fmt.Errorf("wait for cloud-init: %w", err)
		}
		if err := lxdutils.WaitForNetwork(lxd, vmName, ctx); err != nil {
			return fmt.Errorf("wait for network: %w", err)
		}
		return nil
	})
}

func startBuilderVmFromLocalImage(lxd lxdClient.InstanceServer, vmName string, localAlias string) (ok bool) {
	return startVm(lxd, vmName, api.InstanceSource{
		Type:  "image",
		Alias: localAlias,
	})
}

func installTwiggAndTwiggRunner(lxd lxdClient.InstanceServer, vmName string) (ok bool) {
	const maxTries = 3
	return Run("Install tw and twigg-runner", 2*time.Minute, maxTries, func(ctx context.Context) error {
		if err := pushFile(lxd, vmName, *pathToTwiggRunnerBinary, "/usr/local/bin/twigg-runner", 0755); err != nil {
			return fmt.Errorf("push twigg-runner: %w", err)
		}
		if err := pushFile(lxd, vmName, *pathToTwBinary, "/usr/local/bin/tw", 0755); err != nil {
			return fmt.Errorf("push tw: %w", err)
		}
		return nil
	})
}
func installGo(lxd lxdClient.InstanceServer, vmName string) (ok bool) {
	const maxTries = 3
	return Run("Install go", 10*time.Minute, maxTries, func(ctx context.Context) error {
		return execInInstance(ctx, lxd, vmName, []string{"snap", "install", "go", "--classic"})
	})
}
func aptUpdate(lxd lxdClient.InstanceServer, vmName string) (ok bool) {
	const maxTries = 3
	return Run("Update apt", 15*time.Minute, maxTries, func(ctx context.Context) error {
		return execBashInInstance(ctx, lxd, vmName, "apt-get update -y")
	})
}
func installNodeAndNpm(lxd lxdClient.InstanceServer, vmName string) (ok bool) {
	const maxTries = 3
	return Run("Install node and npm", 15*time.Minute, maxTries, func(ctx context.Context) error {
		if err := execBashInInstance(ctx, lxd, vmName, "apt-get install -y nodejs"); err != nil {
			return err
		}
		return execBashInInstance(ctx, lxd, vmName, "apt-get install -y npm")
	})
}
func installBunWithNpm(lxd lxdClient.InstanceServer, vmName string) (ok bool) {
	const maxTries = 3
	return Run("Install bun", 15*time.Minute, maxTries, func(ctx context.Context) error {
		return execBashInInstance(ctx, lxd, vmName, "npm install -g bun")
	})
}
func installDocker(lxd lxdClient.InstanceServer, vmName string) (ok bool) {
	const maxTries = 3
	return Run("Install docker", 10*time.Minute, maxTries, func(ctx context.Context) error {
		script := `
# Add Docker's official GPG key:
apt-get update
apt-get install -y ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

# Add the repository to Apt sources:
cat > /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
		`
		return execBashInInstance(ctx, lxd, vmName, script)
	})
}
func resetAndShrinkVmImage(srv lxdClient.InstanceServer, vmName string) (ok bool) {
	const maxTries = 3
	return Run("Reset and shrink image", 2*time.Minute, maxTries, func(ctx context.Context) error {
		cmds := [][]string{
			{"truncate", "-s", "0", "/etc/machine-id"},
			{"rm", "-f", "/var/lib/dbus/machine-id"},
			{"cloud-init", "clean", "--logs", "--seed"},
			{"bash", "-c", "rm -f /etc/ssh/ssh_host_*"},
			{"apt-get", "clean"},
			{"bash", "-c", "rm -rf /var/lib/apt/lists/*"},
		}
		for _, cmd := range cmds {
			if err := execInInstance(ctx, srv, vmName, cmd); err != nil {
				return err
			}
		}
		return nil
	})
}
func execBashInInstance(ctx context.Context, srv lxdClient.InstanceServer, instance string, bash string) error {
	return execInInstance(ctx, srv, instance, []string{"bash", "-c", fmt.Sprintf("set -euo pipefail\nDEBIAN_FRONTEND=noninteractive\n%s", bash)})
}
func execInInstance(ctx context.Context, srv lxdClient.InstanceServer, instance string, cmd []string) error {
	dataDone := make(chan bool, 1)
	op, err := srv.ExecInstance(instance, api.InstanceExecPost{
		Command:     cmd,
		WaitForWS:   true,
		Interactive: false,
		Environment: map[string]string{
			"DEBIAN_FRONTEND": "noninteractive",
		},
	}, &lxdClient.InstanceExecArgs{
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		DataDone: dataDone,
	})
	if err != nil {
		return fmt.Errorf("exec %v: %w", cmd, err)
	}
	if err := lxdutils.WaitOpOrTimeout(op, ctx, ""); err != nil {
		return fmt.Errorf("exec %v: %w", cmd, err)
	}
	<-dataDone
	if retCode, ok := op.Get().Metadata["return"].(float64); ok && retCode != 0 {
		return fmt.Errorf("exec %v: exit code %d", cmd, int(retCode))
	}
	return nil
}
func stopVmAndPublishImage(srv lxdClient.InstanceServer, vmName string, imageAlias string) (ok bool) {
	const maxTries = 3
	ok = Run("Stop VM running tmp image", 2*time.Minute, maxTries, func(ctx context.Context) error {
		op, err := srv.UpdateInstanceState(vmName, api.InstanceStatePut{
			Action:  "stop",
			Timeout: -1, // -1 = graceful shutdown;
			Force:   false,
		}, "")
		if err != nil {
			return err
		}
		return op.WaitContext(ctx)
	})
	if !ok {
		return
	}
	ok = Run("Publish image", 15*time.Minute, maxTries, func(ctx context.Context) error {
		op, err := srv.CreateImage(api.ImagesPost{
			Source: &api.ImagesPostSource{
				Type: "instance",
				Name: vmName,
			},
			Aliases: []api.ImageAlias{
				{Name: imageAlias},
			},
		}, nil)
		if err != nil {
			return err
		}
		return op.WaitContext(ctx)
	})
	return
}

func pushFile(srv lxdClient.InstanceServer, instance, localPath, remotePath string, mode int) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return srv.CreateInstanceFile(instance, remotePath, lxdClient.InstanceFileArgs{
		Content: f,
		Mode:    mode,
		Type:    "file",
	})
}