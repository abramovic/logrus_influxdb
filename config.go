package logrus_influxdb

import (
	"os"
	"time"
)

type Config struct {
	// InfluxDB Configurations
	Host      string `json:"influxdb_host"`
	Port      int    `json:"influxdb_port"`
	Database  string `json:"influxdb_database"`
	Username  string `json:"influxdb_username"`
	Password  string `json:"influxdb_password"`
	UseHTTPs  bool   `json:"influxdb_usehttps"`
	Precision string `json:"influxdb_precision"`

	// Logrus tags
	Tags []string `json:"logrus_tags"`

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
	if c.BatchInterval < 0 {
		c.BatchInterval = defaultBatchInterval
	}
	if c.BatchCount < 0 {
		c.BatchCount = defaultBatchCount
	}
}
