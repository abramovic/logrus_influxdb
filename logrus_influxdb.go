package logrus_influxdb

import (
	"fmt"
	"os"
	"sync"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/sirupsen/logrus"
)

var (
	defaultHost          = "localhost"
	defaultPort          = 8086
	defaultDatabase      = "logrus"
	defaultBatchInterval = 5 * time.Second
	defaultMeasurement   = "logrus"
	defaultBatchCount    = 200
	defaultPrecision     = "ns"
	defaultSyslog        = false
)

// InfluxDBHook delivers logs to an InfluxDB cluster.
type InfluxDBHook struct {
	sync.Mutex                       // TODO: we should clean up all of these locks
	client                           influxdb.Client
	precision, database, measurement string
	tagList                          []string
	batchP                           influxdb.BatchPoints
	lastBatchUpdate                  time.Time
	batchInterval                    time.Duration
	batchCount                       int
	syslog                           bool
	facility                         string
	facilityCode                     int
	appName                          string
	version                          string
	minLevel                         string
}

// NewInfluxDB returns a new InfluxDBHook.
func NewInfluxDB(config *Config, clients ...influxdb.Client) (hook *InfluxDBHook, err error) {
	if config == nil {
		config = &Config{}
	}
	config.defaults()

	var client influxdb.Client
	if len(clients) == 0 {
		client, err = hook.newInfluxDBClient(config)
		if err != nil {
			return nil, fmt.Errorf("NewInfluxDB: Error creating InfluxDB Client, %v", err)
		}
	} else if len(clients) == 1 {
		client = clients[0]
	} else {
		return nil, fmt.Errorf("NewInfluxDB: Error creating InfluxDB Client, %d is too many influxdb clients", len(clients))
	}

	// Make sure that we can connect to InfluxDB
	_, _, err = client.Ping(5 * time.Second) // if this takes more than 5 seconds then influxdb is probably down
	if err != nil {
		return nil, fmt.Errorf("NewInfluxDB: Error connecting to InfluxDB, %v", err)
	}

	hook = &InfluxDBHook{
		client:        client,
		database:      config.Database,
		measurement:   config.Measurement,
		tagList:       config.Tags,
		batchInterval: config.BatchInterval,
		batchCount:    config.BatchCount,
		precision:     config.Precision,
		syslog:        config.Syslog,
		facility:      config.Facility,
		facilityCode:  config.FacilityCode,
		appName:       config.AppName,
		version:       config.Version,
		minLevel:      config.MinLevel,
	}

	err = hook.autocreateDatabase()
	if err != nil {
		return nil, err
	}
	go hook.handleBatch()

	return hook, nil
}

func parseSeverity(level string) (string, int) {
	switch level {
	case "info":
		return "info", 6
	case "error":
		return "err", 3
	case "debug":
		return "debug", 7
	case "panic":
		return "panic", 0
	case "fatal":
		return "crit", 2
	case "warning":
		return "warning", 4
	}

	return "none", -1
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func (hook *InfluxDBHook) hasMinLevel(level string) bool {
	if len(hook.minLevel) > 0 {
		if hook.minLevel == "debug" {
			return true
		}

		if hook.minLevel == "info" {
			return stringInSlice(level, []string{"info", "warning", "error", "fatal", "panic"})
		}

		if hook.minLevel == "warning" {
			return stringInSlice(level, []string{"warning", "error", "fatal", "panic"})
		}

		if hook.minLevel == "error" {
			return stringInSlice(level, []string{"error", "fatal", "panic"})
		}

		if hook.minLevel == "fatal" {
			return stringInSlice(level, []string{"fatal", "panic"})
		}

		if hook.minLevel == "panic" {
			return level == "panic"
		}

		return false
	}

	return true
}

// Fire adds a new InfluxDB point based off of Logrus entry
func (hook *InfluxDBHook) Fire(entry *logrus.Entry) (err error) {
	if hook.hasMinLevel(entry.Level.String()) {
		measurement := hook.measurement
		if result, ok := getTag(entry.Data, "measurement"); ok {
			measurement = result
		}

		tags := make(map[string]string)
		data := make(map[string]interface{})

		if hook.syslog {
			hostname, err := os.Hostname()

			if err != nil {
				return err
			}

			severity, severityCode := parseSeverity(entry.Level.String())

			tags["appname"] = hook.appName
			tags["facility"] = hook.facility
			tags["host"] = hostname
			tags["hostname"] = hostname
			tags["severity"] = severity

			data["facility_code"] = hook.facilityCode
			data["message"] = entry.Message
			data["procid"] = os.Getpid()
			data["severity_code"] = severityCode
			data["timestamp"] = entry.Time.UnixNano()
			data["version"] = hook.version
		} else {
			// If passing a "message" field then it will be overridden by the entry Message
			entry.Data["message"] = entry.Message

			// Set the level of the entry
			tags["level"] = entry.Level.String()
			// getAndDel and getAndDelRequest are taken from https://github.com/evalphobia/logrus_sentry
			if logger, ok := getTag(entry.Data, "logger"); ok {
				tags["logger"] = logger
			}

			for k, v := range entry.Data {
				data[k] = v
			}

			for _, tag := range hook.tagList {
				if tagValue, ok := getTag(entry.Data, tag); ok {
					tags[tag] = tagValue
					delete(data, tag)
				}
			}
		}

		pt, err := influxdb.NewPoint(measurement, tags, data, entry.Time)
		if err != nil {
			return fmt.Errorf("Fire: %v", err)
		}

		return hook.addPoint(pt)
	}

	return nil
}

func (hook *InfluxDBHook) addPoint(pt *influxdb.Point) (err error) {
	hook.Lock()
	defer hook.Unlock()
	if hook.batchP == nil {
		err = hook.newBatchPoints()
		if err != nil {
			return fmt.Errorf("Error creating new batch: %v", err)
		}
	}
	hook.batchP.AddPoint(pt)

	// if the number of batch points are less than the batch size then we don't need to write them yet
	if len(hook.batchP.Points()) < hook.batchCount {
		return nil
	}
	return hook.writePoints()
}

// writePoints writes the batched log entries to InfluxDB.
func (hook *InfluxDBHook) writePoints() (err error) {
	if hook.batchP == nil {
		return nil
	}
	err = hook.client.Write(hook.batchP)
	// Note: the InfluxDB client doesn't give us any good way to determine the reason for
	// a failure (bad syntax, invalid type, failed connection, etc.), so there is no
	// point in retrying a write.  If the write fails, then we're going to clear out the
	// batch, just as we would for a successful write.

	hook.lastBatchUpdate = time.Now().UTC()
	hook.batchP = nil

	// Return the write error (if any).
	return err
}

// we will periodically flush your points to influxdb.
func (hook *InfluxDBHook) handleBatch() {
	if hook.batchInterval == 0 || hook.batchCount == 0 {
		// we don't need to process this if the interval is 0
		return
	}
	for {
		time.Sleep(hook.batchInterval)
		hook.Lock()
		hook.writePoints()
		hook.Unlock()
	}
}

/* BEGIN BACKWARDS COMPATIBILITY */

// NewInfluxDBHook /* DO NOT USE */ creates a hook to be added to an instance of logger and initializes the InfluxDB client
func NewInfluxDBHook(host, database string, tags []string, batching ...bool) (hook *InfluxDBHook, err error) {
	if len(batching) == 1 && batching[0] {
		return NewInfluxDB(&Config{Host: host, Database: database, Tags: tags}, nil)
	}
	return NewInfluxDB(&Config{Host: host, Database: database, Tags: tags, BatchCount: 0}, nil)
}

// NewWithClientInfluxDBHook /* DO NOT USE */ creates a hook and uses the provided influxdb client
func NewWithClientInfluxDBHook(host, database string, tags []string, client influxdb.Client, batching ...bool) (hook *InfluxDBHook, err error) {
	if len(batching) == 1 && batching[0] {
		return NewInfluxDB(&Config{Host: host, Database: database, Tags: tags}, client)
	}
	return NewInfluxDB(&Config{Host: host, Database: database, Tags: tags, BatchCount: 0}, client)
}

/* END BACKWARDS COMPATIBILITY */
