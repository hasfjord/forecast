package influx

import (
	"context"
	"fmt"

	"github.com/hasfjord/forecast/internal/yr"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Token  string `envconfig:"INFLUX_TOKEN" required:"true"`
	URL    string `envconfig:"INFLUX_URL" required:"true"`
	Org    string `envconfig:"INFLUX_ORG" required:"true"`
	Bucket string `envconfig:"INFLUX_BUCKET" required:"true"`
}

type InfluxClient struct {
	i *influxdb2.Client
	w api.WriteAPIBlocking
}

func NewClient(cfg Config) *InfluxClient {
	// influx client does not provide errors, so use recover to add observability
	defer func() {
		if r := recover(); r != nil {
			fields := logrus.Fields{"URL": cfg.URL, "Bucket": cfg.Bucket}
			logrus.WithFields(fields).Errorf("failed to create new influx client: %v", r)
		}
	}()
	client := influxdb2.NewClient(cfg.URL, cfg.Token)

	writeAPI := client.WriteAPIBlocking(cfg.Org, cfg.Bucket)

	return &InfluxClient{
		i: &client,
		w: writeAPI,
	}
}

func (c InfluxClient) WriteForecast(ctx context.Context, forecast yr.Forecast) error {
	tags := map[string]string{}

	// Write the forecast for the next 24 hours to influx:
	for i, sample := range forecast.Properties.Timeseries[:24] {
		tag := fmt.Sprintf("Temperature_%d", i)
		fields := map[string]interface{}{
			tag: sample.Data.Instant.Details.AirTemperature,
		}

		if err := c.w.WritePoint(ctx,
			write.NewPoint("forecast",
				tags,
				fields,
				sample.Time)); err != nil {
			return err
		}
	}

	return nil
}
