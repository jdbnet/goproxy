<script setup>
import { RouterLink } from 'vue-router'
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-6">
    <div>
      <h1 class="text-xl font-semibold">Getting started</h1>
      <p class="mt-1 text-sm text-muted">How the pieces fit together and how to issue your first certificate.</p>
    </div>

    <section class="card space-y-2">
      <h2 class="font-medium">1. First boot</h2>
      <p class="text-sm text-muted">
        Set <code class="text-heading">GOPROXY_ADMIN_USER</code> and
        <code class="text-heading">GOPROXY_ADMIN_PASSWORD</code> on the first start to create the admin account.
        The management UI listens on the address in <code class="text-heading">config.yaml</code> (default
        <code class="text-heading">127.0.0.1:8080</code>). That port is only for the dashboard and API. Proxy traffic uses frontends.
      </p>
      <p class="text-sm text-muted">
        If <code class="text-heading">proxy.yaml</code> is missing, GoProxy creates the directory and an empty file.
        Set <code class="text-heading">acme_email</code> in <code class="text-heading">config.yaml</code> before requesting Let's Encrypt certificates.
      </p>
    </section>

    <section class="card space-y-2">
      <h2 class="font-medium">2. Frontends, routes, backends</h2>
      <ul class="list-disc space-y-1 pl-5 text-sm text-muted">
        <li>
          <RouterLink to="/frontends" class="text-accent">Frontends</RouterLink>
          are listeners (IP + port). Enable HTTPS on :443. IDs are generated for you.
          Set a default redirect, backend, or Force HTTPS for hosts that do not match a route.
          A :80 listener with Force HTTPS as the default covers every hostname. Keep the real routes on :443.
        </li>
        <li>
          <RouterLink to="/routes" class="text-accent">Routes</RouterLink>
          map a domain to a backend. Search the listener, backend, and certificate by address or domain, not by id.
          <strong class="text-heading">HTTPS (decrypt)</strong> terminates TLS at the proxy.
          <strong class="text-heading">TLS passthrough</strong> copies the encrypted stream through.
          Both can share the same 443 listener.
          Choose Forward, Redirect, or Force HTTPS, then only the matching fields appear.
          Forward can be open, IP locked, password protected, or both.
          Request and response headers can be added or stripped (Home Assistant CSP / X-Frame-Options, extra forwarded headers).
          X-Forwarded-Proto, Host, For, and Port are set automatically.
        </li>
        <li>
          <RouterLink to="/backends" class="text-accent">Backends</RouterLink>
          are your apps. One URL per line for HTTPS routes, or <code class="text-heading">host:port</code> for passthrough.
        </li>
      </ul>
    </section>

    <section class="card space-y-2">
      <h2 class="font-medium">3. Certificates</h2>
      <p class="text-sm text-muted">
        Open
        <RouterLink to="/certificates" class="text-accent">Certificates</RouterLink>
        and pick a challenge:
      </p>
      <ul class="list-disc space-y-1 pl-5 text-sm text-muted">
        <li>
          <strong class="text-heading">HTTP-01</strong>: add a frontend on port 80 first. Let's Encrypt will hit
          <code class="text-heading">/.well-known/acme-challenge/</code> on that listener. Cannot issue wildcards.
        </li>
        <li>
          <strong class="text-heading">DNS-01</strong>: choose your DNS provider from the dropdown. Cloudflare needs an API token
          with Zone.DNS Edit. Tokens are stored in the data directory, not in Git. Wildcards work.
        </li>
        <li>
          <strong class="text-heading">Custom PEM</strong>: upload or paste a certificate and private key. Use this for bought or internally issued certs.
        </li>
      </ul>
      <p class="text-sm text-muted">
        After the cert exists, pick it on an HTTPS route. Renewal is automatic; you can also press Renew.
      </p>
    </section>

    <section class="card space-y-2">
      <h2 class="font-medium">4. Typical first site</h2>
      <ol class="list-decimal space-y-1 pl-5 text-sm text-muted">
        <li>Add a listener on <code class="text-heading">0.0.0.0:80</code> (HTTP) and one on <code class="text-heading">0.0.0.0:443</code> with HTTPS.</li>
        <li>Add a backend with your app URL, for example <code class="text-heading">http://127.0.0.1:3000</code>.</li>
        <li>Issue a certificate for the hostname.</li>
        <li>Add a route: domain, HTTPS (decrypt), the 443 listener, that backend, and the certificate.</li>
      </ol>
    </section>

    <section class="card space-y-2">
      <h2 class="font-medium">5. API keys, Git, and graphs</h2>
      <ul class="list-disc space-y-1 pl-5 text-sm text-muted">
        <li>API keys use the same scopes as user roles. The dashboard talks to the same <code class="text-heading">/api/v1</code> API. Docs: <a class="text-accent" href="/api-docs">/api-docs</a>.</li>
        <li>If Git sync is enabled, proxy.yaml is pulled on start and pushed after dashboard changes. DNS tokens stay local.</li>
        <li>Overview graphs are a 1 hour in-memory buffer. Traffic totals and process uptime are since start. Latency is the window average; each backend server also shows request or health-probe latency. All of this resets on restart. Scrape <code class="text-heading">/metrics</code> with Prometheus for durable history.</li>
        <li>Release builds check for a newer binary on startup and replace themselves when the published checksum changes. Disable with <code class="text-heading">update.enabled: false</code> in config.yaml or <code class="text-heading">GOPROXY_NO_UPDATE=1</code>.</li>
      </ul>
    </section>
  </div>
</template>
