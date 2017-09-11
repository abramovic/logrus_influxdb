package logrus_influxdb

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	influxdb "github.com/influxdata/influxdb/client/v2"
)

var (
	defaultHost          = "localhost"
	defaultPort          = 8086
	defaultDatabase      = "logrus"
	defaultBatchInterval = 5 * time.Second
	defaultMeasurement   = "logrus"
	defaultBatchCount    = 200
	defaultPrecision     = "ns"
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
	}

	err = hook.autocreateDatabase()
	if err != nil {
		return nil, err
	}
	go hook.handleBatch()

	return hook, nil
}

// Fire adds a new InfluxDB point based off of Logrus entry
func (hook *InfluxDBHook) Fire(entry *logrus.Entry) (err error) {
	// If passing a "message" field then it will be overridden by the entry Message
	entry.Data["message"] = entry.Message

	measurement := hook.measurement
	if result, ok := getTag(entry.Data, "measurement"); ok {
		measurement = result
	}

	tags := make(map[string]string)
	// Set the level of the entry
	tags["level"] = entry.Level.String()
	// getAndDel and getAndDelRequest are taken from https://github.com/evalphobia/logrus_sentry
	if logger, ok := getTag(entry.Data, "logger"); ok {
		tags["logger"] = logger
	}

	// make a copy of entry.Data
	data := make(map[string]interface{})
	for k, v := range entry.Data {
		data[k] = v
	}

	for _, tag := range hook.tagList {
		if tagValue, ok := getTag(entry.Data, tag); ok {
			tags[tag] = tagValue
			delete(data, tag)
		}
	}

	pt, err := influxdb.NewPoint(measurement, tags, data, entry.Time)
	if err != nil {
		return fmt.Errorf("Fire: %v", err)
	}
	return hook.addPoint(pt)
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

func (hook *InfluxDBHook) writePoints() (err error) {
	err = hook.client.Write(hook.batchP)
	if err != nil {
		return err
	}
	hook.lastBatchUpdate = time.Now().UTC()
	hook.batchP = nil
	return nil
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
