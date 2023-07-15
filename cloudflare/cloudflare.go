package cloudflare

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	PendingCertificate = "pending_validation"
	ActiveCertificate  = "active"
)

var (
	ErrEmptyResponse = errors.New("cloudflare returned nothing")
	ErrNoResult      = errors.New("cloudflare did not return any result in the response")
)

type Credentials struct {
	Token string
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
		"Authorization": {"Bearer " + credz.Token},
		"Content-Type":  {"application/json"},
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

	if holder.Result == nil {
		return "", ErrEmptyResponse
	}

	if len(holder.Result) < 1 {
		return "", ErrNoResult
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
		"Authorization": {"Bearer " + credz.Token},
		"Content-Type":  {"application/json"},
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

	if holder.Result == nil {
		return []ValidationRecords{}, ErrEmptyResponse
	}

	if len(holder.Result) < 1 {
		return []ValidationRecords{}, ErrNoResult
	}

	return holder.Result[0].ValidationRecords, nil
}

func GetCertificatePacksStatus(id string, credz Credentials) (string, error) {
	type APISchema struct {
		Result []struct {
			Status string `json:"status,omitempty"`
		} `json:"result"`
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/ssl/certificate_packs?status=all", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header = http.Header{
		"Authorization": {"Bearer " + credz.Token},
		"Content-Type":  {"application/json"},
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

	if holder.Result == nil {
		return "", ErrEmptyResponse
	}

	if len(holder.Result) < 1 {
		return "", ErrNoResult
	}

	switch holder.Result[0].Status {
	case "active":
		return ActiveCertificate, nil
	case "pending_validation":
		return PendingCertificate, nil
	default:
		return "", fmt.Errorf("error while getting the certificate packs status: status '%s' is unknown",
			holder.Result[0].Status)
	}
}
