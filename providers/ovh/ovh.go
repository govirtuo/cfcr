package ovh

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ovh/go-ovh/ovh"
	"github.com/rs/zerolog"
)

// OVHProvider is a struct that implements the Provider interface
type OVHProvider struct {
	Credentials Credentials
	BaseDomain  string
	Subdomain   string
}

// Set of credentials required to request OVH API
type Credentials struct {
	ApplicationKey    string
	ApplicationSecret string
	ConsumerKey       string
}

func getDomainIDs(l zerolog.Logger, basedomain, subdomain string, credz Credentials) ([]string, error) {
	type APISchema []int

	client, err := ovh.NewClient(
		"ovh-eu",
		credz.ApplicationKey,
		credz.ApplicationSecret,
		credz.ConsumerKey,
	)
	if err != nil {
		return []string{}, err
	}

	var a APISchema
	uri := fmt.Sprintf("/domain/zone/%s/record?subDomain=%s", basedomain, subdomain)
	l.Debug().Msgf("sending GET on %s", uri)
	if err := client.Get(uri, &a); err != nil {
		return []string{}, err
	}

	var ret []string
	for _, v := range a {
		ret = append(ret, strconv.Itoa(v))
	}
	return ret, nil
}

func getCorrectSubdomain(d, bd string) string {
	// this should do the following
	//   domain: foobar.com -> subdomain: _acme-challenge
	//   domain: www.foobar.com -> subdomain: _acme-challenge.www
	//   domain: www.staging.foobar.com -> subdomain: _acme-challenge.www.staging
	//
	// tests in ovh_test.go serves as a proof
	subdomain := "_acme-challenge"
	if d != bd {
		subdomain = strings.TrimSuffix("_acme-challenge."+d, "."+bd)
	}
	return subdomain
}

// CreateTXTRecords creates TXT records with the content of txtvalues.
func (p OVHProvider) CreateTXTRecords(l zerolog.Logger, domain string, txtvalues ...string) error {
	subdomain := getCorrectSubdomain(domain, p.BaseDomain)

	client, err := ovh.NewClient(
		"ovh-eu",
		p.Credentials.ApplicationKey,
		p.Credentials.ApplicationSecret,
		p.Credentials.ConsumerKey,
	)
	if err != nil {
		return err
	}

	type CreatePostParams struct {
		SubDomain string `json:"subDomain"`
		FieldType string `json:"fieldType"`
		Target    string `json:"target"`
	}

	for _, v := range txtvalues {
		params := CreatePostParams{
			SubDomain: subdomain,
			FieldType: "TXT",
			Target:    v,
		}
		uri := fmt.Sprintf("/domain/zone/%s/record", p.BaseDomain)
		l.Debug().Msgf("sending POST on %s with params %v", uri, params)
		if err := client.Post(uri, params, nil); err != nil {
			return err
		}
	}
	return nil
}

// CleanTXTRecords removes all _acme-challenge.domain TXT records.
func (p OVHProvider) CleanTXTRecords(l zerolog.Logger, domain string) error {
	subdomain := getCorrectSubdomain(domain, p.BaseDomain)
	l.Info().Msgf("getting IDs for %s TXT records on OVH API", subdomain)
	ids, err := getDomainIDs(l, p.BaseDomain, subdomain, p.Credentials)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		l.Info().Msg("nothing to clean")
		return nil
	}
	l.Debug().Msgf("got domain IDs from OVH: %s", ids)

	client, err := ovh.NewClient(
		"ovh-eu",
		p.Credentials.ApplicationKey,
		p.Credentials.ApplicationSecret,
		p.Credentials.ConsumerKey,
	)
	if err != nil {
		return err
	}

	for _, id := range ids {
		uri := fmt.Sprintf("/domain/zone/%s/record/%s", p.BaseDomain, id)
		l.Debug().Msgf("sending DELETE on %s", uri)
		if err := client.Delete(uri, nil); err != nil {
			return err
		}
	}

	return nil
}

func (p OVHProvider) CheckIfRecordsAlreadyExist(l zerolog.Logger, domain string) (bool, error) {
	subdomain := getCorrectSubdomain(domain, p.BaseDomain)
	l.Info().Msgf("getting IDs for %s TXT records on OVH API", subdomain)
	ids, err := getDomainIDs(l, p.BaseDomain, subdomain, p.Credentials)
	if err != nil {
		return false, err
	}

	return len(ids) != 0, nil
}
