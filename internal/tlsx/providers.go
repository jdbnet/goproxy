package tlsx

type ProviderField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Secret      bool   `json:"secret"`
	Required    bool   `json:"required"`
	Help        string `json:"help,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

type Provider struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Fields []ProviderField `json:"fields"`
}

func DNSProviders() []Provider {
	return []Provider{
		{
			ID:   "cloudflare",
			Name: "Cloudflare",
			Fields: []ProviderField{
				{Key: "CLOUDFLARE_DNS_API_TOKEN", Label: "API token", Secret: true, Required: true, Help: "Zone.DNS Edit token. Preferred over Global API key."},
				{Key: "CLOUDFLARE_EMAIL", Label: "Account email", Help: "Only needed if you use the Global API key instead of a token."},
				{Key: "CLOUDFLARE_API_KEY", Label: "Global API key", Secret: true, Help: "Legacy. Prefer an API token."},
			},
		},
		{
			ID:   "route53",
			Name: "Amazon Route 53",
			Fields: []ProviderField{
				{Key: "AWS_ACCESS_KEY_ID", Label: "Access key ID", Required: true},
				{Key: "AWS_SECRET_ACCESS_KEY", Label: "Secret access key", Secret: true, Required: true},
				{Key: "AWS_REGION", Label: "Region", Placeholder: "us-east-1"},
				{Key: "AWS_HOSTED_ZONE_ID", Label: "Hosted zone ID", Help: "Optional. Lego can discover the zone."},
			},
		},
		{
			ID:   "digitalocean",
			Name: "DigitalOcean",
			Fields: []ProviderField{
				{Key: "DO_AUTH_TOKEN", Label: "API token", Secret: true, Required: true},
			},
		},
		{
			ID:   "hetzner",
			Name: "Hetzner",
			Fields: []ProviderField{
				{Key: "HETZNER_API_KEY", Label: "API token", Secret: true, Required: true},
			},
		},
		{
			ID:   "porkbun",
			Name: "Porkbun",
			Fields: []ProviderField{
				{Key: "PORKBUN_API_KEY", Label: "API key", Secret: true, Required: true},
				{Key: "PORKBUN_SECRET_API_KEY", Label: "Secret API key", Secret: true, Required: true},
			},
		},
		{
			ID:   "godaddy",
			Name: "GoDaddy",
			Fields: []ProviderField{
				{Key: "GODADDY_API_KEY", Label: "API key", Secret: true, Required: true},
				{Key: "GODADDY_API_SECRET", Label: "API secret", Secret: true, Required: true},
			},
		},
		{
			ID:   "namecheap",
			Name: "Namecheap",
			Fields: []ProviderField{
				{Key: "NAMECHEAP_API_USER", Label: "API user", Required: true},
				{Key: "NAMECHEAP_API_KEY", Label: "API key", Secret: true, Required: true},
			},
		},
		{
			ID:   "ovh",
			Name: "OVH",
			Fields: []ProviderField{
				{Key: "OVH_ENDPOINT", Label: "Endpoint", Placeholder: "ovh-eu", Required: true},
				{Key: "OVH_APPLICATION_KEY", Label: "Application key", Required: true},
				{Key: "OVH_APPLICATION_SECRET", Label: "Application secret", Secret: true, Required: true},
				{Key: "OVH_CONSUMER_KEY", Label: "Consumer key", Secret: true, Required: true},
			},
		},
		{
			ID:   "gandiv5",
			Name: "Gandi LiveDNS",
			Fields: []ProviderField{
				{Key: "GANDIV5_PERSONAL_ACCESS_TOKEN", Label: "Personal access token", Secret: true, Required: true},
			},
		},
		{
			ID:   "linode",
			Name: "Linode",
			Fields: []ProviderField{
				{Key: "LINODE_TOKEN", Label: "API token", Secret: true, Required: true},
			},
		},
		{
			ID:   "vultr",
			Name: "Vultr",
			Fields: []ProviderField{
				{Key: "VULTR_API_KEY", Label: "API key", Secret: true, Required: true},
			},
		},
		{
			ID:   "dnsimple",
			Name: "DNSimple",
			Fields: []ProviderField{
				{Key: "DNSIMPLE_OAUTH_TOKEN", Label: "OAuth token", Secret: true, Required: true},
			},
		},
		{
			ID:   "duckdns",
			Name: "Duck DNS",
			Fields: []ProviderField{
				{Key: "DUCKDNS_TOKEN", Label: "Token", Secret: true, Required: true},
			},
		},
		{
			ID:   "netlify",
			Name: "Netlify",
			Fields: []ProviderField{
				{Key: "NETLIFY_TOKEN", Label: "API token", Secret: true, Required: true},
			},
		},
		{
			ID:   "vercel",
			Name: "Vercel",
			Fields: []ProviderField{
				{Key: "VERCEL_API_TOKEN", Label: "API token", Secret: true, Required: true},
			},
		},
		{
			ID:   "bunny",
			Name: "Bunny DNS",
			Fields: []ProviderField{
				{Key: "BUNNY_API_KEY", Label: "API key", Secret: true, Required: true},
			},
		},
		{
			ID:   "desec",
			Name: "deSEC",
			Fields: []ProviderField{
				{Key: "DESEC_TOKEN", Label: "Token", Secret: true, Required: true},
			},
		},
		{
			ID:   "cloudns",
			Name: "ClouDNS",
			Fields: []ProviderField{
				{Key: "CLOUDNS_AUTH_ID", Label: "Auth ID", Required: true},
				{Key: "CLOUDNS_AUTH_PASSWORD", Label: "Auth password", Secret: true, Required: true},
			},
		},
		{
			ID:   "rfc2136",
			Name: "RFC2136 (BIND / nsupdate)",
			Fields: []ProviderField{
				{Key: "RFC2136_NAMESERVER", Label: "Nameserver", Placeholder: "192.0.2.1:53", Required: true},
				{Key: "RFC2136_TSIG_KEY", Label: "TSIG key name", Required: true},
				{Key: "RFC2136_TSIG_SECRET", Label: "TSIG secret", Secret: true, Required: true},
				{Key: "RFC2136_TSIG_ALGORITHM", Label: "TSIG algorithm", Placeholder: "hmac-sha256."},
			},
		},
		{
			ID:   "gcloud",
			Name: "Google Cloud DNS",
			Fields: []ProviderField{
				{Key: "GCE_PROJECT", Label: "Project ID", Required: true},
				{Key: "GCE_SERVICE_ACCOUNT_FILE", Label: "Service account JSON path"},
			},
		},
		{
			ID:   "azure",
			Name: "Azure DNS",
			Fields: []ProviderField{
				{Key: "AZURE_CLIENT_ID", Label: "Client ID", Required: true},
				{Key: "AZURE_CLIENT_SECRET", Label: "Client secret", Secret: true, Required: true},
				{Key: "AZURE_SUBSCRIPTION_ID", Label: "Subscription ID", Required: true},
				{Key: "AZURE_TENANT_ID", Label: "Tenant ID", Required: true},
				{Key: "AZURE_RESOURCE_GROUP", Label: "Resource group", Required: true},
			},
		},
		{
			ID:   "other",
			Name: "Other (lego env vars)",
			Fields: []ProviderField{
				{Key: "LEGO_PROVIDER_NAME", Label: "Provider name", Required: true, Help: "Must match a provider in this list, for example cloudflare or route53.", Placeholder: "cloudflare"},
				{Key: "EXTRA_ENV", Label: "Extra environment variables", Help: "One KEY=value per line. These are set only while issuing or renewing this certificate."},
			},
		},
	}
}
