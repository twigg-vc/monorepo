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

# Twigg Web Setup
1 - Create a user to run the service.
```
adduser twigg --disabled-password
sudo usermod -s /usr/sbin/nologin twigg
sudo passwd -l twigg
```

2 - Allow the twiggers group to run the necessary deploy commands
Open `sudo visudo`
```
%twiggers ALL=(root) NOPASSWD: \
  /bin/systemctl stop twigg-web, \
  /bin/systemctl start twigg-web, \
  /bin/systemctl restart twigg-web, \
  /bin/mv /tmp/twigg-web /bin/twigg-web
```

3 - Create the env file with the proper permissions (only the root user can read or modify it)
```
sudo touch /etc/twigg-web.env
sudo chmod 600 /etc/twigg-web.env
sudo chown root:root /etc/twigg-web.env
```

3.1 - Populate the env file
`/etc/twigg-web.env` must define the same variables as `twigg-web.example.env`, with real values filled in.
- For `TWIGG_MASTER_KEY`, run `openssl rand -base64 32` and use the full result (including any trailing `=`).
- Fill in the rest (DigitalOcean Spaces, Stripe, Google/Azure OAuth client credentials, password salt) with the corresponding production values.

4 - Create twigg-web.service
- `sudo nano /etc/systemd/system/twigg-web.service`
- Paste the contents from `twigg-web-prod.service`
- Reload systemd `sudo systemctl daemon-reload`
- Enable at boot `sudo systemctl enable twigg-web.service`

5 - Install caddy and forward traffic to port 9000
Caddyfile:
```
twigg.vc, www.twigg.vc {
        reverse_proxy localhost:9000
}

twigg.sh, www.twigg.sh {
        redir https://twigg.vc{uri} permanent
}
```

6 - Install zip (used for backups)
```
sudo apt install zip
```