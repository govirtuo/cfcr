package cloudflare

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Credentials struct {
	AuthEmail string
	AuthKey   string
}

type ValidationRecords struct {
	Status   string `json:"status"`
	TxtName  string `json:"txt_name"`
	TxtValue string `json:"txt_value"`
}

// GetZoneID takes a zone name and returns the associated zone ID
func GetZoneID(name string, credz Credentials) (string, error) {
	type APISchema struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header = http.Header{
		"X-Auth-Email": {credz.AuthEmail},
		"X-Auth-Key":   {credz.AuthKey},
		"Content-Type": {"application/json"},
	}

	client := &http.Client{}
	r, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", err
	}

	var holder APISchema
	if err := json.Unmarshal(data, &holder); err != nil {
		return "", err
	}

	return holder.Result[0].ID, nil
}

// GetTXTValues requests Cloudflare API to get the validation records
func GetTXTValues(id string, credz Credentials) ([]ValidationRecords, error) {
	type APISchema struct {
		Result []struct {
			ValidationRecords []ValidationRecords `json:"validation_records,omitempty"`
		} `json:"result"`
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/ssl/certificate_packs?status=all", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []ValidationRecords{}, err
	}

	req.Header = http.Header{
		"X-Auth-Email": {credz.AuthEmail},
		"X-Auth-Key":   {credz.AuthKey},
		"Content-Type": {"application/json"},
	}

	client := &http.Client{}
	r, err := client.Do(req)
	if err != nil {
		return []ValidationRecords{}, err
	}
	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return []ValidationRecords{}, err
	}

	var holder APISchema
	if err := json.Unmarshal(data, &holder); err != nil {
		return []ValidationRecords{}, err
	}

	return holder.Result[0].ValidationRecords, nil
}
