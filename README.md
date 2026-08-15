<div align="center">
  <img src="ui/public/favicon.png" alt="GoProxy" width="128" />

# GoProxy

A reverse proxy and load balancer with a web UI. Point a domain at an app, issue a certificate, and keep going. Built as a single static binary: the dashboard is already inside it.

Think Nginx Proxy Manager for day-to-day use, with HAProxy-style routing when you need it: HTTPS termination and TLS passthrough on the same port, IP locks, basic auth, redirects, and health-checked backends.

</div>

## Getting started

On a Linux server (amd64 or arm64):

```bash
curl -fsSL https://git.jdbnet.co.uk/jamie/goproxy/raw/branch/main/deploy/install.sh | sudo bash
```

That installs the binary, systemd service, `/etc/goproxy`, and `/var/lib/goproxy`, then starts GoProxy.

Open the dashboard at `http://<your-server>:8080`. The dashboard is the admin UI, not your public website.

| Username | Password |
| --- | --- |
| `admin` | `changeme` |

Change the password under **Users**.

## Add a site

Click **Add site** in the sidebar. The wizard walks you through the domain, backend, certificate, and listener. You can create new ones or reuse what you already have.

Point DNS at the server, set the ACME email in **Settings** if you are using Let's Encrypt, and you are done. For TLS passthrough, redirects, IP allow lists, and basic auth, use the individual **Routes** and **Frontends** pages.

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

Installed releases check for a newer binary on startup. If the published checksum differs, GoProxy replaces itself and restarts. A failed check is logged and the current binary keeps running.

Turn it off in `/etc/goproxy/config.yaml`:

```yaml
update:
  enabled: false
```

Or set `GOPROXY_NO_UPDATE=1`. Dev builds (`version` is `dev`) do not self-update unless `update.allow_dev` is true.

[MIT](LICENSE)