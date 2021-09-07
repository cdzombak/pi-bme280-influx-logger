# `pi-bme280-influx-logger`

Raspberry Pi application which logs temperature, humidity, and pressure data from an attached [BME280](https://www.adafruit.com/product/2652) sensor to InfluxDB. I use this at home to keep a history of weather conditions.

## Hardware Setup

Connect the sensor to the 3.3V/ground and I2C pins on the Raspberry Pi. For the Pi Zero (W) and Pi 3B+ or 4B, these pins are:

- Pin 1: 3.3v
- Pin 3: I2C SDA
- Pin 5: I2C SCL
- Pin 6: Ground

[Here's a reference page for the Pi Zero's GPIO pins.](https://pinout.xyz/pinout/io_pi_zero) Those pin numbers _may_ also apply to older/other Pi models; I'm not 100% sure, however, and I don't use any other Pi models at home.

Finally, run `sudo raspi-config` and enable I2C in the Interface Options section.

## Installation

Build a binary for your target architecture. For the Raspberry Pi Zero W, this looks like:

```
env GOOS=linux GOARCH=arm GOARM=6 go build -o ~/tmp/pi-bme280-influx-logger .
```

Copy the resulting binary to your Raspberry Pi (I put it in `/usr/local/bin`).

Copy [`piwx.service`](https://github.com/cdzombak/pi-bme280-influx-logger/blob/main/piwx.service) to your Pi, in `/etc/systemd/system`, and edit it to configure the logger. Enable and start it (replacing `piwx.service` if you're calling your service something different):

```
sudo systemctl daemon-reload
sudo systemctl enable piwx.service
sudo systemctl start piwx.service
```

## Configuration Options

The logger is configured via CLI options:

- `-elevation-meters float`: Elevation in meters. *Required* for accurate mean sea level pressure readings. Default is Ann Arbor (259.08 m).
- `-fast-sample`: Sample faster, mainly useful for debugging.
- `-influx-bucket string`: InfluxDB bucket. Supply a string in the form `database/retention-policy`. For the default retention policy, pass just a database name (without the slash character). *Required.*
- `-influx-password` string: InfluxDB password.
- `-influx-server string`: InfluxDB server, including protocol and port, eg. `http://192.168.1.2:8086`. *Required.*
- `-influx-username string`: InfluxDB username.
- `-log-readings`: Log temperature/humidity/pressure readings to standard error.
- `-measurement-name string`: Value for the measurement name in InfluxDB. Defaults to `pi_wx`.
- `-sensor-name string`: Value for the `sensor_name` tag in InfluxDB. *Required.*
