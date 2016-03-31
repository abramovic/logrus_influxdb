# InfluxDB Hook for Logrus

Feel free to create an issue or send me a pull request if you have any questions, bugs, or suggestions for this library.

- [GoDoc](https://godoc.org/github.com/Abramovic/logrus_influxdb)
- [Examples](https://github.com/Abramovic/logrus_influxdb/tree/master/examples)
- [Logrus](https://github.com/Sirupsen/logrus)
- [InfluxDB](https://influxdb.com)

#### [Contributors](https://github.com/Abramovic/logrus_influxdb/graphs/contributors)

Thank you for creating issues and pull requests!

- [Vincent Serpoul](https://github.com/vincentserpoul)
- [Vlad-Doru Ion](https://github.com/vlad-doru)

## Usage

```go
import (
  "time"
  "github.com/Sirupsen/logrus"
  "github.com/Abramovic/logrus_influxdb"
)
func main() {
  log    := logrus.New()

  config := &logrus_influxdb.Config{
    Host: "localhost",
    Port: 8086,
    Database: "logrus",
    UseHTTPS: false,
    Precision: "ns",
    Tags: []string{"tag1", "tag2"},
    BatchInterval: (5 * time.Second),
    BatchCount: 200, // set to "0" to disable batching
  }

  /*
    Use nil if you want to use the default configurations

    hook, err := logrus_influxdb.NewInfluxDB(nil)
  */

  hook, err := logrus_influxdb.NewInfluxDB(config)
  if err == nil {
    log.Hooks.Add(hook)
  }  
}

```

#### With an existing InfluxDB Client

If you wish to initialize a InfluxDB Hook with an already initialized InfluxDB client, you can use the `NewWithClientInfluxDBHook` constructor:

```go
import (
	"github.com/Abramovic/logrus_influxdb"
	"github.com/Sirupsen/logrus"
	client "github.com/influxdata/influxdb/client/v2"
)

func main() {
	log := logrus.New()

	// In this example we will use the default configurations
	config := &logrus_influxdb.Config{
		Tags: []string{"tag1", "tag2"}, // use the following tags
	}

	// Connect to InfluxDB using the standard client.
	influxClient, _ := client.NewHTTPClient(client.HTTPConfig{
		Addr: "http://localhost:8086",
	})

	hook, err := logrus_influxdb.NewInfluxDB(config, influxClient)
	if err == nil {
		log.Hooks.Add(hook)
	}
}
```

## Behind the scenes

#### Database Handling

When passing an empty string for the InfluxDB database name, we default to "logrus" as the database name.

When initializing the hook we attempt to first see if the database exists. If not, then we try to create it for your automagically.

#### Message Field

We will insert your message into InfluxDB with the field `message` so please make sure not to use that name with your Logrus fields or else it will be overwritten.

#### Special Fields

Some logrus fields have a special meaning in this hook, these are `server_name`, `logger` and `http_request`  (taken from [Sentry Hook](https://github.com/evalphobia/logrus_sentry)).

- `server_name` (also known as hostname) is the name of the server which is logging the event (hostname.example.com)
- `logger` is the part of the application which is logging the event
- `http_request` is the in-coming request (*http.Request)
