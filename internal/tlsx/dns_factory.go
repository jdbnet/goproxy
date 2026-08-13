package tlsx

import (
	"fmt"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns/azure"
	"github.com/go-acme/lego/v4/providers/dns/bunny"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/cloudns"
	"github.com/go-acme/lego/v4/providers/dns/desec"
	"github.com/go-acme/lego/v4/providers/dns/digitalocean"
	"github.com/go-acme/lego/v4/providers/dns/dnsimple"
	"github.com/go-acme/lego/v4/providers/dns/duckdns"
	"github.com/go-acme/lego/v4/providers/dns/gandiv5"
	"github.com/go-acme/lego/v4/providers/dns/gcloud"
	"github.com/go-acme/lego/v4/providers/dns/godaddy"
	"github.com/go-acme/lego/v4/providers/dns/hetzner"
	"github.com/go-acme/lego/v4/providers/dns/linode"
	"github.com/go-acme/lego/v4/providers/dns/namecheap"
	"github.com/go-acme/lego/v4/providers/dns/netlify"
	"github.com/go-acme/lego/v4/providers/dns/ovh"
	"github.com/go-acme/lego/v4/providers/dns/porkbun"
	"github.com/go-acme/lego/v4/providers/dns/rfc2136"
	"github.com/go-acme/lego/v4/providers/dns/route53"
	"github.com/go-acme/lego/v4/providers/dns/vercel"
	"github.com/go-acme/lego/v4/providers/dns/vultr"
)

// newDNSProvider builds only the DNS-01 providers we ship in the UI.
// Do not import github.com/go-acme/lego/v4/providers/dns: that package
// pulls every lego provider and their cloud SDKs into the binary.
func newDNSProvider(name string) (challenge.Provider, error) {
	switch name {
	case "azure":
		return azure.NewDNSProvider()
	case "bunny":
		return bunny.NewDNSProvider()
	case "cloudflare":
		return cloudflare.NewDNSProvider()
	case "cloudns":
		return cloudns.NewDNSProvider()
	case "desec":
		return desec.NewDNSProvider()
	case "digitalocean":
		return digitalocean.NewDNSProvider()
	case "dnsimple":
		return dnsimple.NewDNSProvider()
	case "duckdns":
		return duckdns.NewDNSProvider()
	case "gandiv5":
		return gandiv5.NewDNSProvider()
	case "gcloud":
		return gcloud.NewDNSProvider()
	case "godaddy":
		return godaddy.NewDNSProvider()
	case "hetzner":
		return hetzner.NewDNSProvider()
	case "linode":
		return linode.NewDNSProvider()
	case "namecheap":
		return namecheap.NewDNSProvider()
	case "netlify":
		return netlify.NewDNSProvider()
	case "ovh":
		return ovh.NewDNSProvider()
	case "porkbun":
		return porkbun.NewDNSProvider()
	case "rfc2136":
		return rfc2136.NewDNSProvider()
	case "route53":
		return route53.NewDNSProvider()
	case "vercel":
		return vercel.NewDNSProvider()
	case "vultr":
		return vultr.NewDNSProvider()
	default:
		return nil, fmt.Errorf("unsupported DNS provider %q", name)
	}
}
