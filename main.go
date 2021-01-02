package main

import (
	"fmt"
	"iot-backend/pkg/chtapi"
	"iot-backend/pkg/db"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	_ "github.com/joho/godotenv/autoload"
)

type roomMeasure struct {
	RoomID      string `uri:"roomId" binding:"alphanum,required"`
	Measurement string `uri:"measurement" binding:"alphanum,required"`
}
type rangeOptions struct {
	// TODO add time validator
	RelTime string `form:"range" binding:"alphanum"`
}

func main() {
	sensorData := make(map[string]chtapi.SensorPair)
	for pair := range chtapi.ListAllSensors(32) {
		sensorData[pair.Device.ID+pair.Sensor.ID] = pair
	}
	log.Println(sensorData)

	influxClient := influxdb2.NewClientWithOptions("http://localhost:8086", os.Getenv("INFLUX_TOKEN"),
		influxdb2.DefaultOptions().SetLogLevel(3))
	writeAPI := influxClient.WriteAPI("iot-sensor", "sensor")
	defer func() {
		writeAPI.Flush()
		influxClient.Close()
	}()

	errorsCh := writeAPI.Errors()
	go func() {
		for err := range errorsCh {
			log.Printf("write error: %s\n", err.Error())
		}
	}()

	client := TestClient()
	dataStreams := chtapi.SubAllSensors(client, 100)
	db.SaveToInflux(dataStreams, writeAPI)

	if os.Getenv("PRODUCE_FAKE_DATA") == "true" {
		go chtapi.PubAllFakeData(client)
	}

	queryAPI := influxClient.QueryAPI("iot-sensor")

	// API start
	route := gin.New()
	route.Use(gin.Logger())

	api := route.Group("/api")
	{
		api.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
			var msg string
			if err, ok := recovered.(string); ok {
				msg = fmt.Sprintf("error: %s", err)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": msg})
		}))

		api.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, "Home Page")
		})
		api.GET("/building", func(c *gin.Context) {
			data := db.ListBuildings(queryAPI, "3d")
			var buildingData, roomData gin.H
			buildingList := make([]gin.H, 0)
			buildingID := ""
			roomID := int64(-1)
			for _, v := range data {
				if buildingID != v["buildingID"] {
					buildingID = v["buildingID"].(string)
					buildingData = make(gin.H)
					buildingList = append(buildingList, buildingData)
					buildingData["buildingName"] = v["buildingName"]
					buildingData["buildingID"] = v["buildingID"]
					buildingData["rooms"] = make([]gin.H, 0)
				}
				if roomID != v["table"] {
					roomID = v["table"].(int64)
					roomData = make(gin.H)
					buildingData["rooms"] = append(buildingData["rooms"].([]gin.H), roomData)
					roomData["RoomID"] = v["RoomID"]
					roomData["isAbnorml"] = make(gin.H)
				}
				pair := sensorData[v["deviceId"].(string)+v["_field"].(string)]
				for _, attr := range pair.Sensor.Attributes {
					if attr.Key == "threshold" {
						if f, err := strconv.ParseFloat(attr.Value, 64); err == nil {
							roomData["isAbnorml"].(gin.H)[pair.Sensor.ID] = v["_value"].(float64) > f
						}
						break
					}
				}

			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": buildingList})
		})
		api.GET("/sensor", func(c *gin.Context) {
			var data []chtapi.SensorPair
			for pair := range chtapi.ListAllSensors(32) {
				data = append(data, pair)
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
		})

		api.GET("/room/:roomId/:measurement/last", func(c *gin.Context) {
			var rm roomMeasure
			options := rangeOptions{RelTime: "3d"}

			defer log.Println(&rm, &options)
			if err := c.ShouldBindUri(&rm); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("%v", err)})
				return
			} else if err := c.ShouldBindQuery(&options); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("%v", err)})
				return
			}

			data := db.LastMeasurement(queryAPI, db.LastMeasurementData{
				Bucket:      "sensor",
				RelTime:     options.RelTime,
				Measurement: rm.Measurement,
				Filters:     map[string]string{"RoomID": rm.RoomID},
			})
			for _, v := range data {
				pair := sensorData[v["deviceId"].(string)+v["_field"].(string)]
				v["unit"] = pair.Sensor.Unit
				for _, attr := range pair.Sensor.Attributes {
					if attr.Key == "threshold" {
						if f, err := strconv.ParseFloat(attr.Value, 64); err == nil {
							v["abnormal"] = v["_value"].(float64) > f
						}
						break
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
		})
	}
	route.Run(":8088")
}
