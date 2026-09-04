# Setup to run twigg-track locally

1. Install Docker

2. Add your user to the Docker group:
```
sudo usermod -aG docker $USER
newgrp docker
```

3. Install LXD (required for VM job support):
```
sudo snap install lxd
sudo lxd init --auto
sudo usermod -aG lxd $USER
newgrp lxd
```

4. Run the test suite from /monorepo/twigg-track:
```
task test
```

This command will build all required dependencies and execute the tests.  
If the tests pass, your environment is correctly set up.

# General server setup:
1.0 - ssh as root (get the IP from digital ocean) ``ssh root@{digital ocean IP}`

1.1 - Create personal users
```
sudo adduser andre --disabled-password
sudo adduser bob --disabled-password
```

1.2 - Create a group
```
sudo groupadd twiggers
```

1.3 - Add them to a twiggers group
```
sudo usermod -aG twiggers andre
sudo usermod -aG twiggers bob
```

2 - Add your ssh keys to authenticate
- Dump them in your local computer with `cat ~/.ssh/id_ed25519.pub`
Paste them in `sudo mkdir -p /home/andre/.ssh && sudo nano /home/andre/.ssh/authorized_keys`
Fix the permissions
```
sudo chown -R andre:andre /home/andre/.ssh
sudo chmod 700 /home/andre/.ssh
sudo chmod 600 /home/andre/.ssh/authorized_keys
```


3 - Test connecting
- `ssh andre@{digital ocean IP}`

4 - Disable root password login, configure firewall and install fail2ban
- `sudo nano /etc/ssh/sshd_config`
- Set the following (it's probably already done but just check):
```
PasswordAuthentication no
```
- Restart ssh `sudo systemctl restart ssh`
- Install fail2ban `apt install fail2ban`
- Setup ufw to deny incoming by default and only allow https and ssh
```
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow ssh
sudo ufw allow 443
sudo ufw enable
sudo ufw status verbose
```
5 - Update all `sudo apt update && sudo apt upgrade -y`
6 - Install unatended upgrades `apt install unattended-upgrades`

# Twigg-Track server setup

1 - Install docker (regular install, not rootless), gVisor, and LXD.
To install LXD:
```
sudo snap install lxd
sudo lxd init --auto
```

2 - Create a user to run the service. Lock it down and add it to the docker and lxd groups.
TODO: note that this requires us to add `track` to the `docker` group, which is not
ideal because `docker` ~= `root`. We may eventually want to use a docker
proxy so that `track` doesnt need to be a member of `docker` (see
https://github.com/Tecnativa/docker-socket-proxy). Then we just need to use the
`OverrideDockerHost` in the config.
```
adduser track --disabled-password
sudo usermod -aG docker track
sudo usermod -aG lxd track
sudo usermod -s /usr/sbin/nologin track
sudo passwd -l track
```

2 - Allow the twiggers group to run the necessary deploy commands
Open `sudo visudo`
```
%twiggers ALL=(root) NOPASSWD: \
  /bin/systemctl stop twigg-track, \
  /bin/systemctl start twigg-track, \
  /bin/systemctl restart twigg-track, \
  /bin/mv /tmp/twigg-track /bin/twigg-track

%twiggers ALL=(track) NOPASSWD: /usr/bin/docker load -i /tmp/twigg-runners.tar
%twiggers ALL=(track) NOPASSWD: /snap/bin/lxc image delete twigg-vm-runner
%twiggers ALL=(track) NOPASSWD: /snap/bin/lxc image import /tmp/twigg-vm-runner.tar.gz --alias twigg-vm-runner

%twiggers ALL=(root) NOPASSWD: /usr/bin/journalctl -u twigg-track -f
```

3 - Create the env file with the proper permissions (only the root user can read or modify it)
```
sudo touch /etc/twigg-track.env
sudo chmod 600 /etc/twigg-track.env
sudo chown root:root /etc/twigg-track.env
```
Populate it with the contents of `twigg-track.env.example`, filling in every variable.

4 - Create twigg-track.service
- `sudo nano /etc/systemd/system/twigg-track.service`
- Paste the contents from `twigg-track-prod.service`
- Reload systemd `sudo systemctl daemon-reload`
- Enable at boot `sudo systemctl enable twigg-track.service`

4.1 - Install the server firewall (nftables: host rules + job network ACLs)
- `sudo nano /etc/twigg-track.nft`
- Paste the contents from `twigg-track.nft`
- `sudo nano /etc/systemd/system/twigg-track-nftables.service`
- Paste the contents from `twigg-track-nftables.service`
- Reload systemd `sudo systemctl daemon-reload`
- Enable at boot `sudo systemctl enable --now twigg-track-nftables.service`
- Note: If the checked-in files change, repeat this step and
  `sudo systemctl reload twigg-track-nftables` (rules swap atomically).
- Note: twigg-track.service Requires= this unit, so the server won't start if the
  rules fail to load.

4.2 - Install the docker/LXD forwarding shim

Docker network config messes with LXD. We must run a couple commands to
alter docker network config to fix that. Create a systemd unit that will always
run them once at boot:
- `sudo nano /etc/systemd/system/twigg-track-docker-user.service`
- Paste the contents from `twigg-track-docker-user.service`
- Reload systemd `sudo systemctl daemon-reload`
- Enable at boot `sudo systemctl enable --now twigg-track-docker-user.service`

5 - Install caddy and forward traffic to port 2000
Caddyfile:
```
track.twigg.vc {
        reverse_proxy localhost:2000
}
```

6 - Disable ufw (IMPORTANT: check this even on existing servers)

Track servers centralize ALL firewall config in `/etc/twigg-track.nft`
(host rules + job-network ACLs). The general server setup enables ufw, but
two firewalls means conflicting rules — on track servers ufw MUST be
disabled:
```
sudo ufw disable
sudo ufw status   # must print "Status: inactive"
```
Notes:
- Do this only AFTER step 4.1, or the server is left with no firewall.