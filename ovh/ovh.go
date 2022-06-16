package ovh

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/ovh/go-ovh/ovh"
)

type Credentials struct {
	ApplicationKey    string
	ApplicationSecret string
	ConsumerKey       string
}

func GetDomainIDs(subdomain string, credz Credentials) ([]string, error) {
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
	uri := fmt.Sprintf("/domain/zone/%s/record?subDomain=%s", os.Getenv("BASE_DOMAIN"), subdomain)
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

func UpdateTXTRecord(id, txtRecord, subdomain string, credz Credentials) error {
	client, err := ovh.NewClient(
		"ovh-eu",
		credz.ApplicationKey,
		credz.ApplicationSecret,
		credz.ConsumerKey,
	)
	if err != nil {
		return err
	}

	type UpdatePutParams struct {
		SubDomain string `json:"subDomain"`
		Target    string `json:"target"`
	}

	params := &UpdatePutParams{
		SubDomain: subdomain,
		Target:    "\"" + txtRecord + "\"",
	}
	uri := fmt.Sprintf("/domain/zone/%s/record/%s", os.Getenv("BASE_DOMAIN"), id)
	if err := client.Put(uri, params, nil); err != nil {
		return err
	}
	return nil
}
