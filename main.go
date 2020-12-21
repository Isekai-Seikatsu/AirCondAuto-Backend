package main

import (
	"fmt"
	"iot-backend/pkg/chtapi"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	cache := make(map[chtapi.SensorPair]chan []byte)

	sp := make(chan chtapi.SensorPair)
	go chtapi.ListAllSensors(sp)

	client := TestClient()
	for v := range sp {
		topic := fmt.Sprintf("/v1/device/%s/sensor/%s/rawdata", v.Device.ID, v.Sensor.ID)
		log.Println(topic)
		cache[v] = make(chan []byte)

		func(payloadChan chan []byte) {
			client.Subscribe(topic, 0, func(c mqtt.Client, m mqtt.Message) {
				payload := m.Payload()
				payloadChan <- payload
			})
		}(cache[v])
	}

	for _, v := range cache {
		go func(payloadChan chan []byte) {
			for payload := range payloadChan {
				log.Println(string(payload))
			}
		}(v)
	}

	<-time.After(5 * time.Second)

}
