package db

import (
	"context"
	"fmt"
	"iot-backend/pkg/chtapi"
	"log"
	"strconv"
	"strings"
	"text/template"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	api "github.com/influxdata/influxdb-client-go/v2/api"
)

// LastMeasurementData data to query LastMeasuremnet
type LastMeasurementData struct {
	Bucket      string
	RelTime     string
	Measurement string
	Filters     map[string]string
}

var lastMeasureTemp *template.Template

func init() {
	lastMeasureTemp = template.Must(template.New("lastMeasure").Parse(`from(bucket: {{.Bucket|printf "%q"}}) |> range(start: -{{.RelTime}}) |> filter(fn: (r) => r._measurement=={{.Measurement | printf "%q"}} {{range $k, $v := .Filters }}and r.{{$k}}=={{$v | printf "%q"}} {{end}}) |> last()`))
}

// SaveToInflux saves to influxdb
func SaveToInflux(dataStreams chtapi.DataStreams, writeAPI api.WriteAPI) {
	for pair, dataChan := range dataStreams {
		go func(pair chtapi.SensorPair, dataChan chan chtapi.Payload) {
			for payload := range dataChan {
				p := influxdb2.NewPointWithMeasurement("AirQuality")
				p.AddTag("deviceId", pair.Device.ID)
				for _, attr := range pair.Device.Attributes {
					// TODO sort tags to improve performance that insert into influxdb
					p.AddTag(attr.Key, attr.Value)
				}
				value, err := strconv.ParseFloat(payload.Value[0], 64)
				if err != nil {
					log.Println(err)
					continue
				}
				p.AddField(pair.Sensor.ID, value)
				p.SetTime(payload.Time)
				writeAPI.WritePoint(p)
				log.Println(pair, payload)
			}
		}(pair, dataChan)
	}
}

// LastMeasurement query the last measurment with tags
func LastMeasurement(queryAPI api.QueryAPI, queryData LastMeasurementData) []map[string]interface{} {
	data := make([]map[string]interface{}, 0)
	var queryBuidler strings.Builder

	lastMeasureTemp.Execute(&queryBuidler, queryData)
	query := queryBuidler.String()
	log.Println(query)
	result, err := queryAPI.Query(context.Background(), query)
	if err == nil {
		// Iterate over query response
		for result.Next() {
			// Notice when group key has changed
			if result.TableChanged() {
				fmt.Printf("table: %s\n", result.TableMetadata().String())
			}
			// Access data
			values := result.Record().Values()
			fmt.Printf("value: %s\n", values)
			data = append(data, values)
		}
		// check for an error
		if result.Err() != nil {
			fmt.Printf("query parsing error: %s\n", result.Err().Error())
		}
	} else {
		panic(err)
	}
	return data

}
