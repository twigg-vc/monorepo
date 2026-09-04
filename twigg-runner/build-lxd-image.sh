#!/bin/bash
set -e

# Clean up any leftover builder instance and previous image
lxc delete builder --force 2>/dev/null || true
lxc image delete twigg-vm-runner 2>/dev/null || true

# Start from Canonical's image
lxc launch ubuntu:24.04 builder --vm

# Wait for the LXD agent to come up, then for cloud-init to finish
echo "Waiting for VM to boot..."
until lxc exec builder -- cloud-init status --wait 2>/dev/null; do
    sleep 2
done

# Install Twigg and Twigg runner
lxc file push ./bin/twigg-runner-linux builder/usr/local/bin/twigg-runner --mode=0755
lxc file push ../tw/bin/tw_linux_latest builder/usr/local/bin/tw --mode=0755

# Install packages
lxc exec builder -- bash << 'EOF'
set -e
export DEBIAN_FRONTEND=noninteractive

# Helper to retry command a few times
retry() {
    local n=0
    until [ $n -ge 5 ]; do
        "$@" && return 0
        n=$((n+1))
        echo "Attempt $n failed, retrying in 10s..."
        sleep 10
    done
    return 1
}

# Update
retry apt-get update -y

# Install go
TERM=dumb retry snap install go --classic

# Install podman and docker compatibility layer
retry apt-get install -y podman podman-docker
EOF

# Reset and shrink
lxc exec builder -- bash << 'EOF'
set -e
# Reset machine identity
truncate -s 0 /etc/machine-id
rm -f /var/lib/dbus/machine-id
# Clean cloud-init so it re-runs on first boot of derived VMs
cloud-init clean --logs --seed
# Remove SSH host keys (regenerated on first boot)
rm -f /etc/ssh/ssh_host_*
# Shrink the image
apt-get clean
rm -rf /var/lib/apt/lists/*
EOF

# Publish and remove the builder instance
lxc stop builder
lxc publish builder --alias twigg-vm-runner
lxc delete builder
