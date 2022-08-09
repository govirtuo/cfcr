package providers

import (
	"github.com/govirtuo/cfcr/config"
	"github.com/rs/zerolog"
)

const (
	NONE = iota
	OVH
)

var List = []string{
	NONE: "none",
	OVH:  "ovh",
}

// Provider is an interface that represents a provider that can see its TXT records
// being updated to allow Cloudflare to renew certs
type Provider interface {
	UpdateTXTRecords(l zerolog.Logger, domain string, txtvalues ...string) error
}

func ProviderToUse(c config.Config) string {
	if c.Auth.OVH.AppKey != "" && c.Auth.OVH.AppSecret != "" && c.Auth.OVH.ConsumerKey != "" {
		return List[OVH]
	}
	return List[NONE]
}
