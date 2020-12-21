package main

import (
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTSubProxy A MQTT subcriber proxy with one unique topic to many gorutines.
type MQTTSubProxy struct {
	Client *mqtt.Client
	Topic  string
	Subs   []<-chan []byte
}

// TestClient client imformation in development
func TestClient() mqtt.Client {
	options := mqtt.NewClientOptions()
	options.AddBroker("tcp://iot.cht.com.tw:1883")
	options.SetUsername(os.Getenv("CHT_PROJECT_CK"))
	options.SetPassword(os.Getenv("CHT_PROJECT_CK"))

	client := mqtt.NewClient(options)
	token := client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	return client
}
