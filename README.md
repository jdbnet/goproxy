<div align="center">
  <img src="ui/public/favicon.png" alt="GoProxy" width="128" />

# GoProxy

A reverse proxy and load balancer with a web UI. Point a domain at an app, issue a certificate, and keep going. Built as a single static binary: the dashboard is already inside it.

Think Nginx Proxy Manager for day-to-day use, with HAProxy-style routing when you need it: HTTPS termination and TLS passthrough on the same port, IP locks, basic auth, redirects, and health-checked backends.

</div>

## Install

On a Linux host (amd64 or arm64):

```bash
curl -fsSL https://apps.jdbnet.co.uk/goproxy-amd64 -o /tmp/goproxy
# or goproxy-arm64 on ARM
sudo install -m 755 /tmp/goproxy /usr/local/bin/goproxy
```

Or let the install script drop in systemd, `/etc/goproxy`, and `/var/lib/goproxy`:

```bash
curl -fsSL https://git.jdbnet.co.uk/jamie/goproxy/raw/branch/main/deploy/install.sh | sudo bash
```

## First login

The dashboard is **not** your public website. It is the admin UI, default `http://127.0.0.1:8080`.

On first start, if no users exist yet, GoProxy creates an admin account:

| Username | Password |
| --- | --- |
| `admin` | `changeme` |

Sign in and change the password under **Users** before exposing the management port.

To use different credentials on first boot only, set both environment variables before starting:

```bash
sudo GOPROXY_ADMIN_USER=admin GOPROXY_ADMIN_PASSWORD='pick-a-strong-password' /usr/local/bin/goproxy /etc/goproxy/config.yaml
```

If you installed the systemd unit, put those two variables in a drop-in and restart:

```bash
sudo systemctl edit goproxy
```

```ini
[Service]
Environment=GOPROXY_ADMIN_USER=admin
Environment=GOPROXY_ADMIN_PASSWORD=pick-a-strong-password
```

```bash
sudo systemctl restart goproxy
```

Then open `http://127.0.0.1:8080` (SSH tunnel if the box is remote). The env vars are only used when the user database is empty; remove them from the unit after the account exists if you want.

Set `acme_email` in `/etc/goproxy/config.yaml` before you request Let's Encrypt certificates.

## Add a site

Do this in the UI. You do not need to edit YAML by hand.

1. **Frontends**: add `0.0.0.0:80` (HTTP) and `0.0.0.0:443` with HTTPS.
2. **Backends**: add your app, one URL per line, for example `http://127.0.0.1:3000`.
3. **Certificates**: issue a cert for the hostname (HTTP-01 needs the port 80 listener; DNS-01 is for wildcards).
4. **Routes**: domain, Forward, HTTPS (decrypt), the 443 listener, that backend, and the certificate.

Force HTTPS, apex-to-www redirects, IP allow lists, and basic auth are options on the route. TLS passthrough is for apps that terminate TLS themselves; those backends use `host:port` instead of `http://`.

Unmatched hosts can use the frontend default: close, redirect, or forward.

## What the dashboard shows

Overview is live traffic, errors, connections, latency, and backend health. Traffic and uptime are since the process started. The graphs are about one hour in memory and reset on restart. Scrape `/metrics` with Prometheus if you want history that survives a reboot.

API docs live at `/api-docs` on the same port as the UI.

## Files

| Path | What it is |
| --- | --- |
| `/etc/goproxy/config.yaml` | Process settings: UI listen address, log level, data dir, ACME email, Git, backups |
| `/etc/goproxy/proxy.yaml` | Listeners, routes, backends, certificates (what the UI edits) |
| `/var/lib/goproxy` | SQLite state, certs, DNS tokens, backups |

The UI listen address in `config.yaml` is only for admin. Public traffic uses the frontends you add.

`log_level` is `error`, `warn`, `info` (default), or `debug`. Per-request access lines are debug only, so a busy site will not fill the journal. Errors, health changes, and startup stay at info. Override with `GOPROXY_LOG_LEVEL`.

## Git sync

Git stores `proxy.yaml` (and anything else in that directory). On startup GoProxy **pulls**. It does not upload your current file first. After that, each UI save commits and pushes.

If the remote already has a `proxy.yaml`, that file replaces the local one and is applied. An empty or starter file on the remote will wipe routes you added in the UI.

If you configured the box first, commit the live `/etc/goproxy/proxy.yaml` to the repo, then enable Git and restart:

```yaml
git:
  enabled: true
  url: git@git.example.com:you/goproxy-config.git
  branch: main
  auth: ssh
  key_path: /etc/goproxy/deploy_key
```

`auth: token` uses `token` over HTTPS. A failed pull keeps the local file and logs an error.

## Auto update

Installed releases check `https://apps.jdbnet.co.uk/goproxy-amd64` (or `goproxy-arm64`) on startup. If the published checksum differs, the binary replaces itself and restarts. A failed check is logged and the current binary keeps running.

Turn it off in `/etc/goproxy/config.yaml`:

```yaml
update:
  enabled: false
```

Or set `GOPROXY_NO_UPDATE=1`. Dev builds (`version` is `dev`) do not self-update unless `update.allow_dev` is true.

## From source

Only needed if you are changing the code. The release binary already includes the UI.

```bash
./build.sh
./goproxy config.yaml
```
