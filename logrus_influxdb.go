package logrus_influxdb

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	influxdb "github.com/influxdb/influxdb/client"
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
	client   *influxdb.Client
	database string
	tags     map[string]string
}

// NewInfluxDBHook creates a hook to be added to an instance of logger and initializes the InfluxDB client
func NewInfluxDBHook(hostname string, database string, tags map[string]string) (*InfluxDBHook, error) {
	// use the default database if we're missing one in the initialization
	if database == "" {
		database = DefaultDatabase
	}

	if tags == nil { // if no tags exist then make an empty map[string]string
		tags = make(map[string]string)
	}

	u, err := url.Parse(fmt.Sprintf("http://%s:%d", hostname, DefaultPort))
	if err != nil {
		return nil, err
	}
	conf := influxdb.Config{
		URL:      *u,
		Username: os.Getenv("INFLUX_USER"), // detect InfluxDB environment variables
		Password: os.Getenv("INFLUX_PWD"),
		Timeout:  100 * time.Millisecond, // Default timeout of 100 milliseconds
	}

	client, err := influxdb.NewClient(conf)
	if err != nil {
		return nil, err
	}

	// Try pinging InfluxDB to see if it's a valid connection
	_, _, err = client.Ping()
	if err != nil {
		return nil, err
	}

	hook := &InfluxDBHook{client, database, tags}

	err = hook.autocreateDatabase()
	if err != nil {
		return nil, err
	}

	return hook, nil
}

// NewWithClientInfluxDBHook creates a hook using an initialized InfluxDB client.
func NewWithClientInfluxDBHook(client *influxdb.Client, database string, tags map[string]string) (*InfluxDBHook, error) {
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

	point := influxdb.Point{
		Measurement: "logrus",
		Tags:        hook.tags, // set the default tags from hook
		Fields:      fields,
		Time:        entry.Time, // use time from Logrus
	}

	// Set the level of the entry
	point.Tags["level"] = entry.Level.String()

	// getAndDel and getAndDelRequest are taken from https://github.com/evalphobia/logrus_sentry
	if logger, ok := getField(entry.Data, "logger"); ok {
		point.Tags["logger"] = logger
	}
	if serverName, ok := getField(entry.Data, "server_name"); ok {
		point.Tags["server_name"] = serverName
	}
	if req, ok := getRequest(entry.Data, "http_request"); ok {
		point.Fields["http_request"] = req
	}

	_, err := hook.client.Write(influxdb.BatchPoints{
		Points:          []influxdb.Point{point},
		Database:        hook.database,
		RetentionPolicy: "default",
	})
	if err != nil {
		return err
	}
	return nil
}

// queryDB convenience function to query the database
func (hook *InfluxDBHook) queryDB(cmd string) ([]influxdb.Result, error) {
	response, err := hook.client.Query(influxdb.Query{
		Command:  cmd,
		Database: hook.database,
	})
	if err != nil {
		return nil, err
	}
	if response.Error() != nil {
		return nil, response.Error()
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
	_, err = hook.queryDB(fmt.Sprintf("create database %s", hook.database))
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
