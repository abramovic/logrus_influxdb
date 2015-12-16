# InfluxDB Hook for Logrus

[InfluxDB](https://influxdb.com) Scalable datastore for metrics, events, and real-time analytics
[GitHub](https://github.com/influxdb/influxdb)

## Usage

#### Basic Usage (without tags)

```go
import (
  "github.com/Sirupsen/logrus"
  "github.com/Abramovic/logrus_influxdb"
)
func main() {
  // The simplest way to connect
  log       := logrus.New()
  hook, err := logrus_influxdb.NewInfluxDBHook("localhost", "my_influxdb_database", nil)
  if err == nil {
    log.Hooks.Add(hook)
  }
}
```

#### Basic Usage (with tags)

```go
import (
  "github.com/Sirupsen/logrus"
  "github.com/Abramovic/logrus_influxdb"
)
func main() {
  log         := logrus.New()

  tagList := []string{"tag1", "tag2"}
  hook, err := logrus_influxdb.NewInfluxDBHook("localhost", "my_influxdb_database", tagList)

  if err == nil {
    log.Hooks.Add(hook)
  }  
}

```

#### With an existing InfluxDB Client

If you wish to initialize a InfluxDB Hook with an already initialized InfluxDB client, you can use the `NewWithClientInfluxDBHook` constructor:

```go
import (
  "net/url"
  "fmt"
  "time"
  "github.com/Sirupsen/logrus"
  "github.com/Abramovic/logrus_influxdb"
  client "github.com/influxdb/influxdb/client"
)

func main() {
  log   := logrus.New()

  /*
    Connect to InfluxDB using the standard client.
    We are ignoring errors in this example
  */
  u, _    := url.Parse(fmt.Sprintf("http://%s:%d", "localhost", 8086)) // default localhost and 8086 port for InfluxDB
  config  := client.Config{
    URL:      *u,
    Timeout:  100 * time.Millisecond, // The InfluxDB default timeout is 0. In this example we're using 100ms.
  }
  conn, _ := client.NewClient(config)

  /*
    Use the InfluxDB client taken from earlier in the application
  */
  hook, err  := logrus_influxdb.NewWithClientInfluxDBHook(conn, "my_influxdb_database", nil)  // no default tags in this example
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
