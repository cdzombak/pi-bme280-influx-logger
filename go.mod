module pi-bme280-influx-logger

go 1.15

//replace gobot.io/x/gobot => github.com/cdzombak/gobot v1.15.1-0.20210318200731-d6454367611d

require (
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/influxdata/influxdb-client-go/v2 v2.2.2
	gobot.io/x/gobot v1.15.0
)
