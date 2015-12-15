package logrus_influxdb

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	influxdb "github.com/influxdb/influxdb/client/v2"
)

const (
	// DefaultHost default InfluxDB hostname
	DefaultHost = "localhost"
	// DefaultPort default InfluxDB port
	DefaultPort = 8086
	// DefaultDatabase default InfluxDB database. We'll only try to use this if one is not provided.
	DefaultDatabase = "logrus"
)

// InfluxDBHook delivers logs to an InfluxDB cluster.
type InfluxDBHook struct {
	client   influxdb.Client
	database string
	tags     map[string]string
}

// NewInfluxDBHook creates a hook to be added to an instance of logger and initializes the InfluxDB client
func NewInfluxDBHook(
	hostname, database string,
	tags map[string]string,
) (*InfluxDBHook, error) {

	if hostname == "" {
		hostname = DefaultHost
	}

	// use the default database if we're missing one in the initialization
	if database == "" {
		database = DefaultDatabase
	}

	if tags == nil { // if no tags exist then make an empty map[string]string
		tags = make(map[string]string)
	}

	client, err := influxdb.NewHTTPClient(influxdb.HTTPConfig{
		Addr:     fmt.Sprintf("http://%s:%d", hostname, DefaultPort),
		Username: os.Getenv("INFLUX_USER"),
		Password: os.Getenv("INFLUX_PWD"),
		Timeout:  100 * time.Millisecond,
	})
	if err != nil {
		fmt.Println("Error creating InfluxDB Client: ", err.Error())
	}
	defer client.Close()

	hook := &InfluxDBHook{client, database, tags}

	err = hook.autocreateDatabase()
	if err != nil {
		return nil, err
	}

	return hook, nil
}

// NewWithClientInfluxDBHook creates a hook using an initialized InfluxDB client.
func NewWithClientInfluxDBHook(
	client influxdb.Client,
	database string,
	tags map[string]string,
) (*InfluxDBHook, error) {
	// use the default database if we're missing one in the initialization
	if database == "" {
		database = DefaultDatabase
	}

	if tags == nil { // if no tags exist then make an empty map[string]string
		tags = make(map[string]string)
	}

	// If the configuration is nil then assume default configurations
	if client == nil {
		return NewInfluxDBHook(DefaultHost, database, tags)
	}
	return &InfluxDBHook{client, database, tags}, nil
}

// Fire is called when an event should be sent to InfluxDB
func (hook *InfluxDBHook) Fire(entry *logrus.Entry) error {
	// Merge all of the fields from Logrus as one entry in InfluxDB
	fields := entry.Data

	// If passing a "message" field then it will be overridden by the entry Message
	fields["message"] = entry.Message

	// Create a new point batch
	bp, _ := influxdb.NewBatchPoints(influxdb.BatchPointsConfig{
		Database:        hook.database,
		Precision:       "s",
		RetentionPolicy: "default",
	})

	measurement := "logrus"
	if measurement, ok := getField(entry.Data, "measurement"); ok {
		hook.tags["measurement"] = measurement
	}
	// getAndDel and getAndDelRequest are taken from https://github.com/evalphobia/logrus_sentry
	if logger, ok := getField(entry.Data, "logger"); ok {
		hook.tags["logger"] = logger
	}

	// Set the level of the entry
	hook.tags["level"] = entry.Level.String()

	pt, err := influxdb.NewPoint(
		measurement,
		hook.tags,
		// hook.fields,
		fields,
		entry.Time,
	)
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	bp.AddPoint(pt)

	hook.client.Write(bp)

	return nil
}

// queryDB convenience function to query the database
func (hook *InfluxDBHook) queryDB(cmd string) ([]influxdb.Result, error) {
	var res []influxdb.Result
	response, err := hook.client.Query(influxdb.Query{
		Command:  cmd,
		Database: hook.database,
	})
	if err != nil {
		return nil, response.Error()
	}
	if response.Error() != nil {
		return res, response.Error()
	}

	return response.Results, nil
}

// Return back an error if the database does not exist in InfluxDB
func (hook *InfluxDBHook) databaseExists() error {
	results, err := hook.queryDB("SHOW DATABASES")
	if err != nil {
		return err
	}
	if results == nil || len(results) == 0 {
		return errors.New("Missing results from InfluxDB query response")
	}
	if results[0].Series == nil || len(results[0].Series) == 0 {
		return errors.New("Missing series from InfluxDB query response")
	}

	// This can probably be cleaned up
	for _, value := range results[0].Series[0].Values {
		for _, val := range value {
			if v, ok := val.(string); ok { // InfluxDB returns back an interface. Try to check only the string values.
				if v == hook.database { // If we the database exists, return back nil errors
					return nil
				}
			}
		}
	}
	return errors.New("No matching database can be detected")
}

// Try to detect if the database exists and if not, automatically create one.
func (hook *InfluxDBHook) autocreateDatabase() error {
	err := hook.databaseExists()
	if err == nil {
		return nil
	}

	_, err = hook.queryDB(fmt.Sprintf("CREATE DATABASE %s", hook.database))
	if err != nil {
		return err
	}

	return nil
}

// Try to return a field from logrus
// Taken from Sentry adapter (from https://github.com/evalphobia/logrus_sentry)
func getField(d logrus.Fields, key string) (string, bool) {
	var (
		ok  bool
		v   interface{}
		val string
	)
	if v, ok = d[key]; !ok {
		return "", false
	}

	if val, ok = v.(string); !ok {
		return "", false
	}
	return val, true
}

// Try to return an http request
// Taken from Sentry adapter (from https://github.com/evalphobia/logrus_sentry)
func getRequest(d logrus.Fields, key string) (*http.Request, bool) {
	var (
		ok  bool
		v   interface{}
		req *http.Request
	)
	if v, ok = d[key]; !ok {
		return nil, false
	}
	if req, ok = v.(*http.Request); !ok || req == nil {
		return nil, false
	}
	return req, true
}

// Levels is available logging levels.
func (hook *InfluxDBHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
