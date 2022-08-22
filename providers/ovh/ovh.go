package ovh

import (
	"errors"
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

	if len(a) == 0 {
		return []string{}, errors.New("returned IDs list is empty")
	}

	var ret []string
	for _, v := range a {
		ret = append(ret, strconv.Itoa(v))
	}
	return ret, nil
}

func (p OVHProvider) UpdateTXTRecords(l zerolog.Logger, domain string, txtvalues ...string) error {
	subdomain := getCorrectSubdomain(domain, p.BaseDomain)
	l.Info().Msgf("getting IDs for %s TXT records on OVH API", subdomain)
	ids, err := getDomainIDs(l, p.BaseDomain, subdomain, p.Credentials)
	if err != nil {
		return err
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

	type UpdatePutParams struct {
		SubDomain string `json:"subDomain"`
		Target    string `json:"target"`
		TTL       int    `json:"ttl"`
	}

	// there are 3 cases to handle:
	//    1) more DNS record IDs than TXT records: not an issue, we just need
	//       to loop around txtvalues as there is enough available space for all
	//       of the TXT values to fit
	//    2) same number of IDs and txtvalues, so it's a perfect match
	//    3) more TXT records than DNS record IDs: we will have to create new
	//       DNS entries and loop around txtvalues. Just create what we need

	// this block covers the cases 1) and 2)
	if len(txtvalues) <= len(ids) {
		l.Debug().Msg("less or same number of TXT values than IDs, no need to create new DNS records")
		for i, txtvalue := range txtvalues {
			params := &UpdatePutParams{
				SubDomain: subdomain,
				Target:    txtvalue,
				TTL:       120,
			}
			uri := fmt.Sprintf("/domain/zone/%s/record/%s", p.BaseDomain, ids[i])
			l.Debug().Msgf("sending PUT on %s with params %v", uri, params)
			if err := client.Put(uri, params, nil); err != nil {
				return err
			}
		}
	}

	// this block covers the case 3)
	if len(txtvalues) > len(ids) {
		l.Debug().Msg("more TXT values than available DNS TXT records")
		type CreatePostParams struct {
			FieldType string `json:"fieldType"`
			SubDomain string `json:"subDomain"`
			Target    string `json:"target"`
			TTL       int    `json:"ttl"`
		}
		numToCreate := len(txtvalues) - len(ids)
		l.Debug().Msgf("about to create %d DNS TXT records", numToCreate)
		for i := 0; i < numToCreate; i++ {
			// create the TXT entries
			params := &CreatePostParams{
				FieldType: "TXT",
				SubDomain: subdomain,
				Target:    txtvalues[i],
				TTL:       120,
			}
			uri := fmt.Sprintf("/domain/zone/%s/record", p.BaseDomain)
			l.Debug().Msgf("sending POST on %s with params %v", uri, params)
			if err := client.Post(uri, params, nil); err != nil {
				return err
			}
		}
		l.Debug().Msgf("%d new DNS TXT records successfully created, updating those remaining", numToCreate)

		// we start at 0 + numToCreate, as the created DNS records have the first n TXT values
		for i := numToCreate; i < len(txtvalues); i++ {
			params := &UpdatePutParams{
				SubDomain: subdomain,
				Target:    txtvalues[i],
				TTL:       120,
			}
			uri := fmt.Sprintf("/domain/zone/%s/record/%s", p.BaseDomain, ids[i])
			l.Debug().Msgf("sending PUT on %s with params %v", uri, params)
			if err := client.Put(uri, params, nil); err != nil {
				return err
			}
		}
	}
	return nil
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
