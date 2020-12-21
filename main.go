package main

import (
	"iot-backend/pkg/chtapi"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	client := TestClient()

	dataStream := chtapi.SubAllSensors(client, 100)
	dataStream.LogAll()
}
