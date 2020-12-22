package chtapi

import (
	"encoding/json"
	"net/http"
	"os"
)

// Device cht iot device
type Device struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Attributes []Attribute `json:"attributes"`
}

// Attribute additional key value pair
type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ListDevices list cht iot devices
func ListDevices() (devices []Device, err error) {
	req, err := http.NewRequest("GET", "https://iot.cht.com.tw/iot/v1/device", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("CK", os.Getenv("CHT_PROJECT_CK"))
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&devices); err != nil {
		return nil, err
	}
	return
}
