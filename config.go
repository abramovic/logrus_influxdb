package logrus_influxdb

import (
	"os"
	"time"
)

// Config handles InfluxDB configuration, Logrus tags and batching inserts to InfluxDB
type Config struct {
	// InfluxDB Configurations
	Host      string        `json:"influxdb_host"`
	Port      int           `json:"influxdb_port"`
	Timeout   time.Duration `json:"influxdb_timeout"`
	Database  string        `json:"influxdb_database"`
	Username  string        `json:"influxdb_username"`
	Password  string        `json:"influxdb_password"`
	UseHTTPS  bool          `json:"influxdb_https"`
	Precision string        `json:"influxdb_precision"`

	// Enable syslog format for chronograf logviewer usage
	Syslog       bool   `json:"syslog_enabled"`
	Facility     string `json:"syslog_facility"`
	FacilityCode int    `json:"syslog_facility_code"`
	AppName      string `json:"syslog_app_name"`
	Version      string `json:"syslog_app_version"`

	// Minimum level for push
	MinLevel string `json:"syslog_min_level"`

	// Logrus tags
	Tags []string `json:"logrus_tags"`

	// Defaults
	Measurement string `json:"measurement"`

	// Batching
	BatchInterval time.Duration `json:"batch_interval"` // Defaults to 5s.
	BatchCount    int           `json:"batch_count"`    // Defaults to 200.
}

// Set the default configurations
func (c *Config) defaults() {
	if c.Host == "" {
		c.Host = defaultHost
	}
	if c.Port == 0 {
		c.Port = defaultPort
	}
	if c.Timeout == 0 {
		c.Timeout = 100 * time.Millisecond
	}
	if c.Database == "" {
		c.Database = defaultDatabase
	}
	if c.Username == "" {
		c.Username = os.Getenv("INFLUX_USER")
	}
	if c.Password == "" {
		c.Password = os.Getenv("INFLUX_PWD")
	}
	if c.Precision == "" {
		c.Precision = "ns"
	}
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if c.Measurement == "" {
		c.Measurement = defaultMeasurement
	}
	if c.BatchInterval < 0 {
		c.BatchInterval = defaultBatchInterval
	}
	if c.BatchCount < 0 {
		c.BatchCount = defaultBatchCount
	}
}
