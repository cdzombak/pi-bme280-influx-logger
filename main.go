package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"golang.org/x/exp/io/i2c"

	"github.com/avast/retry-go"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/quhar/bme280"
)

const (
	defaultSampleInterval = 1 * time.Minute
	debugSampleInterval = 10 * time.Second
	influxTimeout  = 5 * time.Second
	influxAttempts = 3
)

// DegreesCToF converts the given measurement of Celsius degrees to Fahrenheit.
func DegreesCToF(degC float64) float64 {
	return degC*1.8 + 32.0
}

// PascalsToMillibar converts the given pressure, in pascals, to millibars.
func PascalsToMillibar(pa float64) float64 {
	return pa / 100.0
}

// PascalsToInHg converts the given pressure, in pascals, to inches of mercury.
func PascalsToInHg(pa float64) float64 {
	return pa * 0.0002953
}

// MSLP adjusts the given raw pressure, in pascals, at the given altitude, in meters,
// to mean sea level pressure. Ref: https://www.weather.gov/bou/pressure_definitions
func MSLP(rawPressurePa, altitudeMeter float64) float64 {
	return rawPressurePa / math.Pow(1.0-(altitudeMeter/44330.0), 5.255)
}

// DewPointF approximates the dew point (in degrees F) given the current
// temperature (in Fahrenheit) and relative humidity.
func DewPointF(tempF, humidity float64) float64 {
	return tempF - ((100.0 - humidity) * (9.0 / 25.0))
}

// IndoorHumidityRecommendation returns the maximum recommended indoor relative
// humidity percentage for the given outdoor temperature (in degrees F).
func IndoorHumidityRecommendation(outdoorTempF float64) int {
	if outdoorTempF >= 50 {
		return 50
	}
	if outdoorTempF >= 40 {
		return 45
	}
	if outdoorTempF >= 30 {
		return 40
	}
	if outdoorTempF >= 20 {
		return 35
	}
	if outdoorTempF >= 10 {
		return 30
	}
	if outdoorTempF >= 0 {
		return 25
	}
	if outdoorTempF >= -10 {
		return 20
	}
	return 15
}

func main() {
	var influxServer = flag.String("influx-server", "", "InfluxDB server, including protocol and port, eg. 'http://192.168.1.1:8086'. Required.")
	var influxUser = flag.String("influx-username", "", "InfluxDB username.")
	var influxPass = flag.String("influx-password", "", "InfluxDB password.")
	var influxBucket = flag.String("influx-bucket", "", "InfluxDB bucket. Supply a string in the form 'database/retention-policy'. For the default retention policy, pass just a database name (without the slash character). Required.")
	var sensorName = flag.String("sensor-name", "", "Value for the sensor_name tag in InfluxDB. Required.")
	var measurementName = flag.String("measurement-name", "pi_wx", "Value for the measurement name in InfluxDB. Defaults to 'pi_wx'.")
	var logResults = flag.Bool("log-readings", false, "Log temperature/humidity/pressure readings to standard error.")
	var elevation = flag.Float64("elevation-meters", 259.08, "Elevation in meters. Required for accurate mean sea level pressure readings. Default is Ann Arbor ;)")
	var debugFastMode = flag.Bool("fast-sample", false, "Sample at a fast sample rate, for debugging.")
	flag.Parse()
	if *influxServer == "" || *influxBucket == "" {
		fmt.Println("-influx-bucket and -influx-server must be supplied.")
		os.Exit(1)
	}
	if *sensorName == "" {
		fmt.Println("-sensor-name must be supplied.")
		os.Exit(1)
	}

	authString := ""
	if *influxUser != "" || *influxPass != "" {
		authString = fmt.Sprintf("%s:%s", *influxUser, *influxPass)
	}
	influxClient := influxdb2.NewClient(*influxServer, authString)
	ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
	defer cancel()
	health, err := influxClient.Health(ctx)
	if err != nil {
		log.Fatalf("failed to check InfluxDB health: %v", err)
	}
	if health.Status != "pass" {
		log.Fatalf("InfluxDB did not pass health check: status %s; message '%s'", health.Status, *health.Message)
	}
	influxWriteApi := influxClient.WriteAPIBlocking("", *influxBucket)

	d, err := i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, bme280.I2CAddr)
	if err != nil {
		panic(err)
	}

	sensor := bme280.New(d)
	err = sensor.Init()
	if err != nil {
		log.Fatalf("failed to initialize BME280: %s", err)
	}

	sampleInterval := defaultSampleInterval
	if *debugFastMode {
		sampleInterval = debugSampleInterval
	}

	ticker := time.NewTicker(sampleInterval)
	for {
		select {
		case <-ticker.C:
			tempC, rawPressurePa, humidity, err := sensor.EnvData()
			if err != nil {
				log.Fatalf("failed to read from BME280: %s", err)
			}

			tempF := DegreesCToF(tempC)
			dewPointF := DewPointF(tempF, humidity)
			indoorHumidityRec := IndoorHumidityRecommendation(tempF)
			pressure := MSLP(rawPressurePa, *elevation)
			pressureMb := PascalsToMillibar(pressure)
			pressureIn := PascalsToInHg(pressure)

			if *logResults {
				log.Printf("temp: %.1f degF", tempF)
				log.Printf("humidity: %.1f%%; dew point: %.1f degF", humidity, dewPointF)
				log.Printf("pressure (MSLP): %.1f mB (%.2f inHg)", pressureMb, pressureIn)
				log.Printf("max. recommended indoor humidity: %d%%", indoorHumidityRec)
			}

			point := influxdb2.NewPoint(
				*measurementName,
				map[string]string{"sensor_name": *sensorName}, // tags
				map[string]interface{}{
					"temperature_f":                   tempF,
					"dew_point_f":                     dewPointF,
					"recommended_max_indoor_humidity": indoorHumidityRec,
					"temperature_c":                   tempC,
					"humidity":                        humidity,
					"raw_pressure_pa":                 rawPressurePa,
					"pressure_pa":                     pressure,
					"pressure_mb":                     pressureMb,
					"pressure_inHg":                   pressureIn,
				}, // fields
				time.Now(),
			)
			if err := retry.Do(
				func() error {
					ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
					defer cancel()
					return influxWriteApi.WritePoint(ctx, point)
				},
				retry.Attempts(influxAttempts),
			); err != nil {
				log.Printf("failed to write point to influx: %v", err)
			}
		}
	}
}
