package yr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/sirupsen/logrus"
)

type Forecast struct {
	Type       string     `json:"type"`
	Geometry   Geometry   `json:"geometry"`
	Properties Properties `json:"properties"`
}

type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type Properties struct {
	Meta struct {
		Updated_at time.Time         `json:"updated_at"`
		Units      map[string]string `json:"units"`
	} `json:"meta"`
	Timeseries []struct {
		Time time.Time `json:"time"`
		Data struct {
			Instant struct {
				Details ForecastDetails `json:"details"`
			} `json:"instant"`
		} `json:"data"`
	} `json:"timeseries"`
}

type ForecastDetails struct {
	AirPressureAtSeaLevel      float64 `json:"air_pressure_at_sea_level"`
	AirTemperature             float64 `json:"air_temperature"`
	AirTemperature10Percentile float64 `json:"air_temperature_percentile_10"`
	AirTemperature90Percentile float64 `json:"air_temperature_percentile_90"`
	CloudAreaFraction          float64 `json:"cloud_area_fraction"`
	CloudAreaFractionHigh      float64 `json:"cloud_area_fraction_high"`
	CloudAreaFractionLow       float64 `json:"cloud_area_fraction_low"`
	CloudAreaFractionMedium    float64 `json:"cloud_area_fraction_medium"`
	DewPointTemperature        float64 `json:"dew_point_temperature"`
	FogAreaFraction            float64 `json:"fog_area_fraction"`
	RelativeHumidity           float64 `json:"relative_humidity"`
	UltraVioletIndexClearSky   float64 `json:"ultraviolet_index_clear_sky"`
	WindFromDirection          float64 `json:"wind_from_direction"`
	WindSpeed                  float64 `json:"wind_speed"`
	WindSpeedOfGust            float64 `json:"wind_speed_of_gust"`
	WindSpeed10Percentile      float64 `json:"wind_speed_percentile_10"`
	WindSpeed90Percentile      float64 `json:"wind_speed_percentile_90"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	BaseURL   string
	UserAgent string
	HTTPClient
}

func NewClient(cfg Config, HTTPClient HTTPClient) *Client {
	return &Client{
		BaseURL:    cfg.BaseURL,
		UserAgent:  cfg.UserAgent,
		HTTPClient: HTTPClient,
	}
}

type Config struct {
	BaseURL   string `envconfig:"YR_URL" default:"https://api.met.no/weatherapi/locationforecast/2.0/"`
	UserAgent string `envconfig:"YR_USER_AGENT" default:"yr-go"`
}

func (c *Client) makeRequest(ctx context.Context, Method string, URL string, params map[string]string, result interface{}) (int, error) {
	// build http-request
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("dtrack failed to build request: %s %s: %w", Method, URL, err)
	}

	// add user agent to header
	request.Header.Set("User-Agent", c.UserAgent)

	// add and encode query parameters
	parameters := request.URL.Query()
	for key, value := range params {
		logrus.Debugf("yr: adding query parameter: %s=%s", key, value)
		parameters.Add(key, value)
	}
	request.URL.RawQuery = parameters.Encode()
	logrus.Debugf("yr: request: %s", request.URL.String())
	logrus.Debug("make request")

	// make request
	res, err := c.Do(request)
	if err != nil && res != nil {
		logrus.WithField("result", res).Info("yr: request made with error")
		return res.StatusCode, fmt.Errorf("dtrack: failed request: %s %s: %s: %w", Method, URL, res.Status, err)
	}
	if res == nil {
		return 500, fmt.Errorf("yr: result is null: %w", err)
	}

	response, _ := httputil.DumpRequest(request, true)

	logrus.WithField("request", string(response)).Debug("yr: request made")

	// close body after reading
	defer func() {
		logrus.Debug("closing body")
		if res.Body != nil {
			_, _ = io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
		}
	}()
	// if status code is not 200 return the status code received, no error, the calling function can decide how to handle it
	logrus.Debug("checking status")
	if res.StatusCode != 200 {
		return res.StatusCode, nil
	}

	err = json.NewDecoder(res.Body).Decode(result)
	if err != nil && !errors.Is(err, io.EOF) {
		return http.StatusInternalServerError, fmt.Errorf("yr: failed to unmarshal result: %w", err)
	}

	return res.StatusCode, nil
}

func (c *Client) GetCompleteForecast(ctx context.Context, lat, lon, altitude float64) (Forecast, error) {
	params := map[string]string{
		"lat":      fmt.Sprintf("%0.4f", lat),
		"lon":      fmt.Sprintf("%0.4f", lon),
		"altitude": fmt.Sprintf("%0.f", altitude),
	}

	URL := fmt.Sprintf("%s/complete", c.BaseURL)

	forecast := Forecast{}
	status, err := c.makeRequest(ctx, http.MethodGet, URL, params, &forecast)
	if err != nil {
		return forecast, fmt.Errorf("yr: failed to get complete forecast: %w", err)
	}
	if status != http.StatusOK {
		return forecast, fmt.Errorf("yr: failed to get complete forecast: wrong status returned: %d", status)
	}
	return forecast, nil
}
