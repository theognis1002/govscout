# GovScout VPS Deployment Runbook

End-to-end guide for deploying GovScout to a single Linux VPS. Target audience: internal use by a few users. Covers build, systemd, HTTPS via Caddy (CNAME-friendly), continuous SQLite backups with Litestream, upgrades, and day-2 ops.

---

## 1. Architecture

```
                  ┌───────────────────────────────────────┐
                  │               VPS                     │
  Internet ──▶ :443 ──▶ Caddy ──▶ :8080 ──▶ govscout (Go) │
                  │                           │           │
                  │                           ▼           │
                  │                    /opt/govscout/     │
                  │                    govscout.db (WAL)  │
                  │                           │           │
                  │                    Litestream (sidecar)
                  │                           │           │
                  └───────────────────────────┼───────────┘
                                              ▼
                                     S3-compatible bucket
                                     (continuous replication)
```

One binary, SQLite on local disk, Caddy terminates TLS, Litestream streams DB changes to object storage. Daily sync runs via systemd timer.

---

## 2. VPS sizing

Minimum:

- 1 vCPU
- 1 GB RAM
- 25 GB SSD
- Ubuntu 24.04 LTS (or Debian 12)

GovScout, Caddy, and Litestream together idle well under 150 MB RAM.

**On a 1 GB box, add 2 GB of swap before building on the VPS** — `go build` alone can push the box into OOM-kill territory, and the same is true of any other Node/Bun-bundled installers you might later run there:

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
sudo sysctl vm.swappiness=10
echo 'vm.swappiness=10' | sudo tee /etc/sysctl.d/99-swap.conf
```

---

## 3. DNS — CNAME setup

You want `alert.example.com` → this VPS.

**Option A (recommended for VPS with stable IP):** A record

```
alert.example.com.   A   203.0.113.42
```

**Option B (CNAME to a hostname):**

```
alert.example.com.   CNAME   my-vps.example.net.
```

Both work with Caddy's automatic HTTPS.

### SSL / TLS

Caddy obtains a Let's Encrypt certificate automatically using the **HTTP-01** challenge, which hits the VPS on port **80** regardless of whether the DNS record is A or CNAME. **No wildcard, no DNS-01, no manual certbot needed.**

Prereqs for Caddy to provision a cert:

1. DNS has propagated (`dig +short alert.example.com` returns the VPS IP).
2. Ports 80 and 443 are open on the VPS firewall.
3. Nothing else is bound to :80 or :443.

If later you want wildcard certs or to hide the origin behind Cloudflare proxied mode, you'll switch to DNS-01 — out of scope here.

---

## 4. Initial server setup

SSH in as root (or a sudo user your provider created).

```bash
# Create a non-root admin user (skip if you already have one)
adduser deploy
usermod -aG sudo deploy
rsync --archive --chown=deploy:deploy ~/.ssh /home/deploy

# Harden SSH
sed -i 's/^#*PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
systemctl restart ssh

# Base packages
apt update && apt upgrade -y
apt install -y ufw unattended-upgrades curl ca-certificates debian-keyring debian-archive-keyring apt-transport-https gnupg sqlite3

# Firewall
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

# Automatic security updates
dpkg-reconfigure -plow unattended-upgrades
```

Reconnect as `deploy` from here on. All following commands assume `sudo` where needed.

---

## 5. Build the binary

### Option A — build locally, copy up (preferred)

From your dev machine, in the repo root:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o govscout-linux-amd64 ./cmd/govscout
scp govscout-linux-amd64 deploy@alert.example.com:/tmp/govscout
```

### Option B — build on the VPS

Ubuntu 24.04's `apt` Go is 1.22, which is too old. Grab Go 1.23+ from go.dev:

```bash
# On the VPS
curl -sLO https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
sudo ln -sf /usr/local/go/bin/go /usr/local/bin/go

sudo apt install -y git
git clone https://github.com/theognis1002/govscout /tmp/govscout-src
cd /tmp/govscout-src
CGO_ENABLED=0 go build -o /tmp/govscout ./cmd/govscout
```

You'll want the 2 GB swap from §2 before this step on a 1 GB box.

---

## 6. Install layout

```bash
# System user for the app
sudo useradd --system --home /opt/govscout --shell /usr/sbin/nologin govscout
sudo mkdir -p /opt/govscout
sudo mv /tmp/govscout /opt/govscout/govscout
sudo chmod +x /opt/govscout/govscout
```

### Create `.env`

```bash
sudo tee /opt/govscout/.env >/dev/null <<EOF
SAMGOV_API_KEY=your_samgov_key_here
AUTH_SECRET=$(openssl rand -hex 32)
GOVSCOUT_DB=/opt/govscout/govscout.db
PORT=8080
RESEND_API_KEY=your_resend_key_here
RESEND_FROM_EMAIL=GovScout <alerts@yourdomain.com>
TEST_EMAIL_TO=you@example.com
EOF

sudo chown -R govscout:govscout /opt/govscout
sudo chmod 600 /opt/govscout/.env
```

Running CLI commands (e.g. `useradd`, `testemail`) manually: the binary loads `.env` from its working directory, so always `cd /opt/govscout` first:

```bash
cd /opt/govscout
sudo -u govscout /opt/govscout/govscout useradd --username admin@example.com --password 'CHANGE_ME' --admin
sudo -u govscout /opt/govscout/govscout testemail
```

Copy the repo's systemd files to `/etc/systemd/system/` (see next section) before starting.

---

## 7. Systemd units

The repo already ships three units in `systemd/`. Copy them:

```bash
# From the cloned repo on the VPS, or scp them up from your dev machine
sudo cp systemd/govscout.service         /etc/systemd/system/
sudo cp systemd/govscout-sync.service    /etc/systemd/system/
sudo cp systemd/govscout-sync.timer      /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable --now govscout.service
sudo systemctl enable --now govscout-sync.timer
```

Verify:

```bash
systemctl status govscout
systemctl list-timers govscout-sync.timer
curl -sf http://127.0.0.1:8080/health && echo OK
```

---

## 8. Create the first admin user

```bash
cd /opt/govscout
sudo -u govscout /opt/govscout/govscout useradd \
  --username admin@example.com \
  --password 'CHANGE_ME_STRONG' \
  --admin
```

The `cd /opt/govscout` matters — the binary loads `.env` (for `GOVSCOUT_DB`) from its working directory.

---

## 9. Caddy — reverse proxy + automatic HTTPS

### Install

```bash
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy
```

### Caddyfile

```bash
sudo mkdir -p /var/log/caddy
sudo chown -R caddy:caddy /var/log/caddy

sudo tee /etc/caddy/Caddyfile >/dev/null <<'EOF'
alert.example.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
    log {
        output file /var/log/caddy/alert.log
        format console
    }
}
EOF

sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

`/var/log/caddy` must exist and be writable by the `caddy` user before reload, or the `log` directive will fail with a permission error on startup.

Caddy will fetch a Let's Encrypt cert within seconds once DNS points at the box. Watch it happen:

```bash
sudo journalctl -u caddy -f
```

Test:

```bash
curl -I https://alert.example.com/health
```

---

## 10. Litestream — continuous SQLite backups

Litestream streams every write from `govscout.db` to object storage. You get point-in-time recovery with RPO measured in seconds.

### Install

```bash
LITESTREAM_VERSION=0.3.13
curl -L "https://github.com/benbjohnson/litestream/releases/download/v${LITESTREAM_VERSION}/litestream-v${LITESTREAM_VERSION}-linux-amd64.deb" -o /tmp/litestream.deb
sudo dpkg -i /tmp/litestream.deb
```

### Config

Using an S3-compatible bucket (Cloudflare R2, Backblaze B2, AWS S3 — pick any). Create a bucket named e.g. `govscout-backup` and an access key scoped to it.

```bash
sudo tee /etc/litestream.yml >/dev/null <<'EOF'
dbs:
  - path: /opt/govscout/govscout.db
    replicas:
      - type: s3
        bucket: govscout-backup
        path: govscout.db
        endpoint: https://<accountid>.r2.cloudflarestorage.com   # or s3.us-west-002.backblazeb2.com, etc.
        region: auto
        access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
EOF
sudo chmod 600 /etc/litestream.yml
```

Credentials go in an environment file so they're not in the YAML:

```bash
sudo tee /etc/default/litestream >/dev/null <<'EOF'
LITESTREAM_ACCESS_KEY_ID=xxx
LITESTREAM_SECRET_ACCESS_KEY=yyy
EOF
sudo chmod 600 /etc/default/litestream
```

The Debian package installs a systemd unit. Point it at the env file:

```bash
sudo systemctl edit litestream
```

Add:

```ini
[Service]
EnvironmentFile=/etc/default/litestream
```

Then:

```bash
sudo systemctl enable --now litestream
sudo systemctl status litestream
```

Verify backup is live (wait ~30s after a write):

```bash
litestream snapshots -config /etc/litestream.yml /opt/govscout/govscout.db
```

---

## 11. Backup restore drill

**Do this once, before you need it.** Spin up a scratch directory:

```bash
sudo systemctl stop govscout
sudo -u govscout litestream restore \
  -config /etc/litestream.yml \
  -o /tmp/restored.db \
  /opt/govscout/govscout.db

# Compare row counts as a sanity check
sqlite3 /opt/govscout/govscout.db 'select count(*) from opportunities;'
sqlite3 /tmp/restored.db           'select count(*) from opportunities;'

sudo systemctl start govscout
```

To do a real recovery after data loss:

```bash
sudo systemctl stop govscout
sudo mv /opt/govscout/govscout.db /opt/govscout/govscout.db.broken
sudo -u govscout litestream restore -config /etc/litestream.yml /opt/govscout/govscout.db
sudo systemctl start govscout
```

---

## 12. Upgrades

Zero-surprise upgrade path:

```bash
# On dev machine
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o govscout-linux-amd64 ./cmd/govscout
scp govscout-linux-amd64 deploy@alert.example.com:/tmp/govscout.new

# On VPS
sudo mv /opt/govscout/govscout /opt/govscout/govscout.prev
sudo mv /tmp/govscout.new /opt/govscout/govscout
sudo chown govscout:govscout /opt/govscout/govscout
sudo chmod +x /opt/govscout/govscout
sudo systemctl restart govscout
sudo journalctl -u govscout -n 50 --no-pager
```

Rollback if something's wrong:

```bash
sudo mv /opt/govscout/govscout.prev /opt/govscout/govscout
sudo systemctl restart govscout
```

Downtime: ~1 second.

---

## 13. Logs & observability

```bash
# App
journalctl -u govscout -f
journalctl -u govscout-sync.service -n 200 --no-pager

# Caddy
journalctl -u caddy -f
tail -f /var/log/caddy/alert.log

# Litestream
journalctl -u litestream -f

# Timer firing history
systemctl list-timers govscout-sync.timer

# Trigger a sync manually
sudo systemctl start govscout-sync.service
```

---

## 14. Health check

```bash
curl -sf https://alert.example.com/health && echo OK
```

Wire this into any external uptime monitor (UptimeRobot, Betterstack free tier, etc.).

---

## 15. Troubleshooting

**Caddy can't get a certificate**

- `dig +short alert.example.com` — does it resolve to this VPS?
- `sudo ss -tlnp | grep -E ':80|:443'` — is anything else holding those ports?
- `sudo ufw status` — are 80/443 allowed?
- `journalctl -u caddy -n 100` — look for ACME errors.

**`govscout` won't start**

- `journalctl -u govscout -n 100` — usually missing `.env` vars or bad DB path.
- Check `.env` perms: `ls -l /opt/govscout/.env` should be `600 govscout govscout`.

**Sync failing**

- `journalctl -u govscout-sync.service -n 200` — 429s mean SAM.gov rate-limited you. The sync command handles this gracefully; it will resume next run.
- Verify `SAMGOV_API_KEY` is valid. Multiple comma-separated keys help.

**DB appears locked**

- Shouldn't happen with WAL + single writer, but if it does: `sudo systemctl restart govscout` and check the journal. Never delete `govscout.db-wal` or `govscout.db-shm` while the app is running.

**Litestream shows no snapshots**

- `systemctl status litestream` — running?
- Check credentials in `/etc/default/litestream`.
- `litestream replicas -config /etc/litestream.yml` should show the replica.

---

## 16. Security checklist

- [ ] Root SSH login disabled, password auth off
- [ ] UFW enabled, only 22/80/443 open
- [ ] `unattended-upgrades` enabled
- [ ] `AUTH_SECRET` is 32+ random chars (via `openssl rand -hex 32`)
- [ ] `/opt/govscout/.env` is `chmod 600`, owned by `govscout`
- [ ] `/etc/default/litestream` is `chmod 600`
- [ ] `govscout.db` is not in a web-served directory (it isn't — it's in `/opt/govscout`)
- [ ] First admin password rotated from any placeholder
- [ ] Backups verified with a real restore drill (section 11)
- [ ] HTTPS confirmed via `curl -I https://alert.example.com/health`

---

## 17. Quick reference — all paths

| Thing                  | Path                                       |
| ---------------------- | ------------------------------------------ |
| Binary                 | `/opt/govscout/govscout`                   |
| Env file               | `/opt/govscout/.env`                       |
| Database               | `/opt/govscout/govscout.db`                |
| Web systemd unit       | `/etc/systemd/system/govscout.service`     |
| Sync systemd unit      | `/etc/systemd/system/govscout-sync.service`|
| Sync timer             | `/etc/systemd/system/govscout-sync.timer`  |
| Caddy config           | `/etc/caddy/Caddyfile`                     |
| Litestream config      | `/etc/litestream.yml`                      |
| Litestream credentials | `/etc/default/litestream`                  |
| Caddy logs             | `/var/log/caddy/alert.log`                 |
