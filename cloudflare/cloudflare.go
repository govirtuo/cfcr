package cloudflare

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	PendingCertificate       = "pending_validation"
	ActiveCertificate        = "active"
	ValidationTimedOut       = "validation_timed_out"
	InitializingCertificates = "initializing"
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	data, err := io.ReadAll(r.Body)
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	data, err := io.ReadAll(r.Body)
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

// GetCertificatePacksStatus returns the status, the cert pack ID and an error
func GetCertificatePacksStatus(id string, credz Credentials) (string, string, error) {
	type APISchema struct {
		Result []struct {
			Status string `json:"status,omitempty"`
			ID     string `json:"id,omitempty"`
		} `json:"result"`
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/ssl/certificate_packs?status=all", id)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}

	req.Header = http.Header{
		"Authorization": {"Bearer " + credz.Token},
		"Content-Type":  {"application/json"},
	}

	client := &http.Client{}
	r, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer r.Body.Close()

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return "", "", err
	}

	var holder APISchema
	if err := json.Unmarshal(data, &holder); err != nil {
		return "", "", err
	}

	if holder.Result == nil {
		return "", "", ErrEmptyResponse
	}

	if len(holder.Result) < 1 {
		return "", ErrNoResult
	}

	switch holder.Result[0].Status {
	case ActiveCertificate:
		return ActiveCertificate, holder.Result[0].ID, nil
	case PendingCertificate:
		return PendingCertificate, holder.Result[0].ID, nil
	case ValidationTimedOut:
		return ValidationTimedOut, holder.Result[0].ID, nil
	case InitializingCertificates:
		return InitializingCertificates, holder.Result[0].ID, nil
	default:
		return "", "", fmt.Errorf("error while getting the certificate packs status: status '%s' is unknown",
			holder.Result[0].Status)
	}
}

// https://developers.cloudflare.com/ssl/edge-certificates/advanced-certificate-manager/manage-certificates/#restart-validation
// https://developers.cloudflare.com/api/operations/certificate-packs-restart-validation-for-advanced-certificate-manager-certificate-pack
func TriggerCertificatesValidation(id, certPackId string, credz Credentials) error {
	type APISchema struct {
		Errors []interface{} `json:"errors"`
		Result struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"result"`
		Success bool `json:"success"`
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/ssl/certificate_packs/%s", id, certPackId)
	req, err := http.NewRequest(http.MethodPatch, url, nil)
	if err != nil {
		return err
	}

	req.Header = http.Header{
		"Authorization": {"Bearer " + credz.Token},
		"Content-Type":  {"application/json"},
	}

	client := &http.Client{}
	r, err := client.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	var holder APISchema
	if err := json.Unmarshal(data, &holder); err != nil {
		return err
	}

	if !holder.Success {
		return fmt.Errorf("error while triggering the certificates renewal: %s", holder.Errors...)
	}

	if holder.Result.Status != InitializingCertificates {
		return fmt.Errorf("unexpected status, expected: %s, got: %s", InitializingCertificates, holder.Result.Status)
	}

	return nil
}
