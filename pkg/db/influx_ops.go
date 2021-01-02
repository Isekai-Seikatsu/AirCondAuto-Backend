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

// RangeMeasurementData data to query RangeMeasurement
type RangeMeasurementData struct {
	RoomID  string
	RelTime string
	Every   string
	AggFunc string
	LimitN  int
}

var lastMeasureTemp *template.Template
var listBuildingTemp *template.Template
var rangeMeasureTemp *template.Template

func init() {
	lastMeasureTemp = template.Must(template.New("lastMeasure").Parse(`from(bucket: {{.Bucket|printf "%q"}}) |> range(start: -{{.RelTime}}) |> filter(fn: (r) => r._measurement=={{.Measurement | printf "%q"}} {{range $k, $v := .Filters }}and r.{{$k}}=={{$v | printf "%q"}} {{end}}) |> last()`))
	listBuildingTemp = template.Must(template.New("listBuilding").Parse(
		`from(bucket: "sensor")
|> range(start: -{{.}})
|> filter(fn: (r) => exists r.buildingID)
|> group(columns: ["buildingID", "RoomID", "_field"])
|> last()
|> group(columns: ["buildingID"])`))
	rangeMeasureTemp = template.Must(template.New("rangeMeasure").Parse(
		`from(bucket: "sensor")
|> range(start: -{{.RelTime}})
|> filter(fn: (r) => r["_measurement"] == "AirQuality" and r["RoomID"] == {{.RoomID | printf "%q"}})
|> aggregateWindow(every: {{.Every}}, fn: {{.AggFunc}}, createEmpty: false)
|> limit(n: {{.LimitN}})`))

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

// ExecuteQuery execute query and return flatten result
func ExecuteQuery(queryAPI api.QueryAPI, query string) []map[string]interface{} {
	result, err := queryAPI.Query(context.Background(), query)
	data := make([]map[string]interface{}, 0)
	if err == nil {
		for result.Next() {
			values := result.Record().Values()
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

// LastMeasurement query the last measurment with tags
func LastMeasurement(queryAPI api.QueryAPI, queryData LastMeasurementData) []map[string]interface{} {
	var queryBuidler strings.Builder

	lastMeasureTemp.Execute(&queryBuidler, queryData)
	query := queryBuidler.String()
	log.Println(query)
	return ExecuteQuery(queryAPI, query)
}

// ListBuildings list all datas with building tag
func ListBuildings(queryAPI api.QueryAPI, relTime string) []map[string]interface{} {
	var queryBuidler strings.Builder

	listBuildingTemp.Execute(&queryBuidler, relTime)
	query := queryBuidler.String()
	log.Println(query)
	return ExecuteQuery(queryAPI, query)
}

// WindowAggregativeMeasurement query the measurement that window aggregated
func WindowAggregativeMeasurement(queryAPI api.QueryAPI, queryData RangeMeasurementData) []map[string]interface{} {
	var queryBuidler strings.Builder

	rangeMeasureTemp.Execute(&queryBuidler, queryData)
	query := queryBuidler.String()
	log.Println(query)
	return ExecuteQuery(queryAPI, query)
}
