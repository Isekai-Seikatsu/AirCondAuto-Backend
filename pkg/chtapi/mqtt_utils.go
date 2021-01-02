package chtapi

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Payload cht mqtt payload format
type Payload struct {
	SensorID string    `json:"id"`
	DeviceID string    `json:"deviceId"`
	Time     time.Time `json:"time"`
	Value    []string  `json:"value"`
}

type pubTestPayload struct {
	SensorID string    `json:"id"`
	Time     time.Time `json:"time"`
	Value    []string  `json:"value"`
}

// DataStreams stores channels listened for a topic from mqtt
type DataStreams map[SensorPair]chan Payload

// SubSensors subscribe specified sensors and return a map of channels that streaming paylods by mqtt.
func SubSensors(client mqtt.Client, bufsize int, sensorPairs <-chan SensorPair) DataStreams {
	dataStreams := make(DataStreams)
	for pair := range sensorPairs {
		topic := fmt.Sprintf("/v1/device/%s/sensor/%s/rawdata", pair.Device.ID, pair.Sensor.ID)
		log.Println(topic)
		dataStreams[pair] = make(chan Payload, bufsize)

		func(payloadChan chan Payload) {
			client.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
				var payload Payload
				if err := json.Unmarshal(m.Payload(), &payload); err != nil {
					log.Fatal(err)
				}
				payloadChan <- payload
			})
		}(dataStreams[pair])
	}
	return dataStreams
}

// SubAllSensors subscribe all sensors and return a map of channels that streaming paylods by mqtt.
func SubAllSensors(client mqtt.Client, bufsize int) DataStreams {
	// TODO update sub sensors list from chtapi to period reload subcribing
	return SubSensors(client, bufsize, ListAllSensors(32))
}

// LogAll logs all data from streams
func (dataStreams DataStreams) LogAll() {
	logChan := make(chan string)

	for _, v := range dataStreams {
		go func(payloadChan chan Payload) {
			for payload := range payloadChan {
				logChan <- fmt.Sprintf("%s", payload)
			}
		}(v)
	}

	for logMsg := range logChan {
		log.Println(logMsg)
	}
}

// PubAllFakeData publish fake datas to do testing
func PubAllFakeData(client mqtt.Client) {
	payloads := make([]pubTestPayload, 1)
	for t := range time.Tick(5 * time.Second) {
		for pair := range ListAllSensors(32) {
			topic := fmt.Sprintf("/v1/device/%s/rawdata", pair.Device.ID)
			payloads[0] = pubTestPayload{
				SensorID: pair.Sensor.ID,
				Time:     t,
				Value:    []string{fmt.Sprintf("%v", rand.Intn(100))},
			}

			buf, err := json.Marshal(payloads)
			if err != nil {
				panic(err)
			}
			log.Println(string(buf))
			client.Publish(topic, 0, false, buf)
		}
	}
}
