package main

import (
	"iot-backend/pkg/chtapi"
	"log"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	client := TestClient()

	dataStream := chtapi.SubAllSensors(client, 100)

	logChan := make(chan string)
	for _, v := range dataStream {
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
