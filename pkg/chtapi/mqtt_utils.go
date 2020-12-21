package chtapi

import (
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// type struct DataStream {

// }

// SubAllSensors subscribe all sensors and return a map of channels that streaming paylods by mqtt.
func SubAllSensors(client mqtt.Client, bufsize int) map[SensorPair]chan []byte {
	dataStream := make(map[SensorPair]chan []byte)
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
