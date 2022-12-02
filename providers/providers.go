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
	// CreateTXTRecords creates the correct number of TXT records, based on the
	// content of txtvalues.
	CreateTXTRecords(l zerolog.Logger, domain string, txtvalues ...string) error
	// CleanTXTRecords removes all the TXT records set on the _acme-challenge.domain
	// domain.
	CleanTXTRecords(l zerolog.Logger, domain string) error
	// CheckIfRecordsAlreadyExist returns true if the domain already has TXT
	// records configured.
	CheckIfRecordsAlreadyExist(l zerolog.Logger, domain string) (bool, error)
}

func ProviderToUse(c config.Config) string {
	if c.Auth.OVH.AppKey != "" && c.Auth.OVH.AppSecret != "" && c.Auth.OVH.ConsumerKey != "" {
		return List[OVH]
	}
	return List[NONE]
}
