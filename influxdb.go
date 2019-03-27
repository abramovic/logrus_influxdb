package logrus_influxdb

import (
	"fmt"

	influxdb "github.com/influxdata/influxdb1-client/v2"
)

// Returns an influxdb client
func (hook *InfluxDBHook) newInfluxDBClient(config *Config) (influxdb.Client, error) {
	protocol := "http"
	if config.UseHTTPS {
		protocol = "https"
	}
	return influxdb.NewHTTPClient(influxdb.HTTPConfig{
		Addr:     fmt.Sprintf("%s://%s:%d", protocol, config.Host, config.Port),
		Username: config.Username,
		Password: config.Password,
		Timeout:  config.Timeout,
	})
}
func (hook *InfluxDBHook) newBatchPoints() (err error) {
	// make sure we're only creating new batch points when we don't already have them
	if hook.batchP != nil {
		return nil
	}
	hook.batchP, err = influxdb.NewBatchPoints(influxdb.BatchPointsConfig{
		Database:  hook.database,
		Precision: hook.precision,
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
func (hook *InfluxDBHook) databaseExists() (err error) {
	results, err := hook.queryDB("SHOW DATABASES")
	if err != nil {
		return err
	}
	if results == nil || len(results) == 0 {
		return fmt.Errorf("Missing results from InfluxDB query response")
	}
	if results[0].Series == nil || len(results[0].Series) == 0 {
		return fmt.Errorf("Missing series from InfluxDB query response")
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
	return fmt.Errorf("No matching database can be detected")
}

// Try to detect if the database exists and if not, automatically create one.
func (hook *InfluxDBHook) autocreateDatabase() (err error) {
	err = hook.databaseExists()
	if err == nil {
		return nil
	}
	_, err = hook.queryDB(fmt.Sprintf("CREATE DATABASE %s", hook.database))
	if err != nil {
		return err
	}
	return nil
}
