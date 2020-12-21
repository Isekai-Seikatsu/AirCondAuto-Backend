package chtapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// Sensor cht iot sensor
type Sensor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Unit string `json:"unit"`
}

// SensorPair pair of device and sensor
type SensorPair struct {
	Device Device
	Sensor Sensor
}

// ListSensors list cht iot sensors of device
func ListSensors(device Device) (sensors []Sensor, err error) {
	url := fmt.Sprintf("https://iot.cht.com.tw/iot/v1/device/%s/sensor", device.ID)
	req, err := http.NewRequest("GET", url, nil)
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

	if err := dec.Decode(&sensors); err != nil {
		return nil, err
	}
	return
}

// ListAllSensors list all sensors with device
func ListAllSensors() <-chan SensorPair {
	resultChan := make(chan SensorPair)
	go func(outChan chan SensorPair) {
		defer close(outChan)
		devices, err := ListDevices()
		if err != nil {
			log.Fatal(err)
			return
		}
		for _, device := range devices {
			sensors, err := ListSensors(device)
			if err != nil {
				log.Fatal(err)
				return
			}
			for _, sensor := range sensors {
				outChan <- SensorPair{device, sensor}
			}
		}
	}(resultChan)
	return resultChan
}
