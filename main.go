package main

import (
	"fmt"
	"iot-backend/pkg/chtapi"
	"iot-backend/pkg/db"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	_ "github.com/joho/godotenv/autoload"
)

type roomMeasure struct {
	RoomID string `uri:"roomId" binding:"alphanum,required"`
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
		api.Use(CORSAllowAll())
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
			roomID := ""
			for _, v := range data {
				if buildingID != v["buildingID"] {
					buildingID = v["buildingID"].(string)
					buildingData = make(gin.H)
					buildingList = append(buildingList, buildingData)
					buildingData["buildingName"] = v["buildingName"]
					buildingData["buildingID"] = v["buildingID"]
					buildingData["rooms"] = make([]gin.H, 0)
				}
				if pair, ok := sensorData[v["deviceId"].(string)+v["_field"].(string)]; ok {
					var deviceRoomID string
					for _, attr := range pair.Device.Attributes {
						if attr.Key == "RoomID" {
							deviceRoomID = attr.Value
						}
					}
					if v["RoomID"] == deviceRoomID {
						if roomID != v["RoomID"] {
							roomID = v["RoomID"].(string)
							roomData = make(gin.H)
							buildingData["rooms"] = append(buildingData["rooms"].([]gin.H), roomData)
							roomData["RoomID"] = v["RoomID"]
							roomData["RoomNumber"] = strings.TrimPrefix(v["RoomID"].(string), v["buildingID"].(string))
							roomData["Abnormal"] = make([]string, 0)
						}
						for _, attr := range pair.Sensor.Attributes {
							if attr.Key == "threshold" {
								if f, err := strconv.ParseFloat(attr.Value, 64); err == nil {
									if v["_value"].(float64) > f {
										roomData["Abnormal"] = append(roomData["Abnormal"].([]string), pair.Sensor.ID)
									}
								}
								break
							}
						}
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
		api.GET("/room/:roomId/AirQuality/range", func(c *gin.Context) {
			var rm roomMeasure
			if err := c.ShouldBindUri(&rm); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": fmt.Sprintf("%v", err)})
				return
			}

			data := db.WindowAggregativeMeasurement(queryAPI, db.RangeMeasurementData{
				RoomID:  rm.RoomID,
				RelTime: "1d",
				Every:   "30m",
				AggFunc: "mean",
				LimitN:  5,
			})
			var (
				rangeData         gin.H
				abnormalThreshold float64
			)
			resultData := make([]gin.H, 0)
			tableIndex := int64(-1)
			for _, v := range data {

				if pair, ok := sensorData[v["deviceId"].(string)+v["_field"].(string)]; ok {
					if v["table"] != tableIndex {
						tableIndex = v["table"].(int64)
						rangeData = make(gin.H)
						resultData = append(resultData, rangeData)
						rangeData["name"] = pair.Sensor.ID
						rangeData["unit"] = pair.Sensor.Unit
						rangeData["time"] = make([]time.Time, 0)
						rangeData["value"] = make([]float64, 0)
						rangeData["abnormal"] = make([]bool, 0)
						for _, attr := range pair.Sensor.Attributes {
							if attr.Key == "threshold" {
								if f, err := strconv.ParseFloat(attr.Value, 64); err == nil {
									abnormalThreshold = f
								}
								break
							}
						}
					}
					rangeData["time"] = append(rangeData["time"].([]time.Time), v["_time"].(time.Time))
					rangeData["value"] = append(rangeData["value"].([]float64), v["_value"].(float64))
					rangeData["abnormal"] = append(rangeData["abnormal"].([]bool), v["_value"].(float64) > abnormalThreshold)
				}
			}

			c.JSON(http.StatusOK, gin.H{"ok": true, "data": resultData})
		})

		api.GET("/room/:roomId/AirQuality/last", func(c *gin.Context) {
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
				Measurement: "AirQuality",
				Filters:     map[string]string{"RoomID": rm.RoomID},
			})
			resultData := make([]gin.H, 0)
			for _, v := range data {
				if pair, ok := sensorData[v["deviceId"].(string)+v["_field"].(string)]; ok {
					v["unit"] = pair.Sensor.Unit
					for _, attr := range pair.Sensor.Attributes {
						if attr.Key == "threshold" {
							if f, err := strconv.ParseFloat(attr.Value, 64); err == nil {
								v["abnormal"] = v["_value"].(float64) > f
							}
							break
						}
					}
					resultData = append(resultData, v)
				}
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": resultData})
		})

		api.GET("/room/:roomId/alert", func(c *gin.Context) {
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
				Measurement: "AirQuality",
				Filters:     map[string]string{"RoomID": rm.RoomID},
			})
			resultData := make([]gin.H, 0)
			var thresholdValue float64
			for _, v := range data {
				alertMsg := ""
				if pair, ok := sensorData[v["deviceId"].(string)+v["_field"].(string)]; ok {
					for _, attr := range pair.Sensor.Attributes {
						if attr.Key == "threshold" {
							if f, err := strconv.ParseFloat(attr.Value, 64); err == nil {
								thresholdValue = f
							}
						} else if attr.Key == "alertMsg" {
							alertMsg = attr.Value
						}
					}
					if v["_value"].(float64) > thresholdValue {
						resultData = append(resultData, gin.H{
							"alert": true,
							"msg":   alertMsg,
						})
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": resultData})
		})
		api.GET("/room/:roomId/controldevice", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true, "data": []gin.H{
				gin.H{"deviceName": "AirCond1", "open": rand.Intn(2) == 1},
				gin.H{"deviceName": "AirCond2", "open": rand.Intn(2) == 1},
			}})
		})
	}

	route.StaticFS("/fonts", http.Dir("./dist/fonts"))
	route.StaticFS("/img", http.Dir("./dist/img"))
	route.StaticFS("/js", http.Dir("./dist/js"))
	route.StaticFS("/css", http.Dir("./dist/css"))
	route.StaticFile("/favicon.ico", "./dist/favicon.ico")
	route.StaticFile("/index.html", "./dist/index.html")
	route.StaticFile("/", "./dist/index.html")

	route.Run(":8088")
}
