package main

import (
	"fmt"
	"iot-backend/pkg/chtapi"
	"iot-backend/pkg/db"
	"log"
	"net/http"
	"os"

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

	queryAPI := influxClient.QueryAPI("iot-sensor")

	// API start
	route := gin.New()
	route.Use(gin.Logger())

	route.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		var msg string
		if err, ok := recovered.(string); ok {
			msg = fmt.Sprintf("error: %s", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": msg})
	}))

	route.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Home Page")
	})
	route.GET("/room/:roomId/:measurement", func(c *gin.Context) {
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
		c.JSON(http.StatusOK, gin.H{"ok": true, "data": data})
	})
	route.Run(":8088")
}
