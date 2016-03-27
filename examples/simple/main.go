package main

import (
	"github.com/Abramovic/logrus_influxdb"
	"github.com/Sirupsen/logrus"
)

func main() {
	log := logrus.New()
	hook, err := logrus_influxdb.NewInfluxDB(nil)
	if err == nil {
		log.Hooks.Add(hook)
	}
}
