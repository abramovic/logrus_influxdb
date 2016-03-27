package logrus_influxdb

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sirupsen/logrus"
)

// Try to return a field from logrus
// Taken from Sentry adapter (from https://github.com/evalphobia/logrus_sentry)
func getTag(d logrus.Fields, key string) (tag string, ok bool) {
	v, ok := d[key]
	if !ok {
		return "", false
	}
	switch vs := v.(type) {
	case fmt.Stringer:
		return vs.String(), true
	case string:
		return vs, true
	case byte:
		return string(vs), true
	case int:
		return strconv.FormatInt(int64(vs), 10), true
	case int32:
		return strconv.FormatInt(int64(vs), 10), true
	case int64:
		return strconv.FormatInt(vs, 10), true
	case uint:
		return strconv.FormatUint(uint64(vs), 10), true
	case uint32:
		return strconv.FormatUint(uint64(vs), 10), true
	case uint64:
		return strconv.FormatUint(vs, 10), true
	default:
		return "", false
	}
	return "", false
}

// Try to return an http request
// Taken from Sentry adapter (from https://github.com/evalphobia/logrus_sentry)
func getRequest(d logrus.Fields, key string) (req *http.Request, ok bool) {
	v, ok := d[key]
	if !ok {
		return nil, false
	}
	req, ok = v.(*http.Request)
	if !ok || req == nil {
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
