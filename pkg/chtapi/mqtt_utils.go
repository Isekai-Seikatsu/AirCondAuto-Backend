package chtapi

import (
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// DataStreams stores channels listened for a topic from mqtt
type DataStreams map[SensorPair]chan []byte

// SubAllSensors subscribe all sensors and return a map of channels that streaming paylods by mqtt.
func SubAllSensors(client mqtt.Client, bufsize int) DataStreams {
	dataStream := make(DataStreams)
	for v := range ListAllSensors() {
		topic := fmt.Sprintf("/v1/device/%s/sensor/%s/rawdata", v.Device.ID, v.Sensor.ID)
		log.Println(topic)
		dataStream[v] = make(chan []byte, bufsize)

		func(payloadChan chan []byte) {
			client.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
				payload := m.Payload()
				payloadChan <- payload
			})
		}(dataStream[v])
	}
	return dataStream
}

// LogAll logs all data from streams
func (dataStreams DataStreams) LogAll() {
	logChan := make(chan string)

	for _, v := range dataStreams {
		go func(payloadChan chan []byte) {
			for payload := range payloadChan {
				logChan <- string(payload)
			}
		}(v)
	}

	for logMsg := range logChan {
		log.Println(logMsg)
	}
}
