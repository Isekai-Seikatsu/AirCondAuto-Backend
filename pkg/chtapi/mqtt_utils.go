package chtapi

import (
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// DataStreams stores channels listened for a topic from mqtt
type DataStreams map[SensorPair]chan []byte

// SubSensors subscribe specified sensors and return a map of channels that streaming paylods by mqtt.
func SubSensors(client mqtt.Client, bufsize int, sensorPairs <-chan SensorPair) DataStreams {
	dataStream := make(DataStreams)
	for pair := range sensorPairs {
		topic := fmt.Sprintf("/v1/device/%s/sensor/%s/rawdata", pair.Device.ID, pair.Sensor.ID)
		log.Println(topic)
		dataStream[pair] = make(chan []byte, bufsize)

		func(payloadChan chan []byte) {
			client.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
				payload := m.Payload()
				payloadChan <- payload
			})
		}(dataStream[pair])
	}
	return dataStream
}

// SubAllSensors subscribe all sensors and return a map of channels that streaming paylods by mqtt.
func SubAllSensors(client mqtt.Client, bufsize int) DataStreams {
	return SubSensors(client, bufsize, ListAllSensors())
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
