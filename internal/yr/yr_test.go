package yr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testServer *httptest.Server
	client     *Client
	ctx        context.Context
)

func TestMain(m *testing.M) {
	ctx = context.Background()
	testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch Path := strings.TrimSpace(r.URL.Path); {
		case Path == "/complete":
			forecast := Forecast{}
			var coordinates []string
			coordinates = append(coordinates, r.URL.Query().Get("lat"))
			coordinates = append(coordinates, r.URL.Query().Get("lon"))
			coordinates = append(coordinates, r.URL.Query().Get("altitude"))

			for _, v := range coordinates {
				var err error
				cord, err := strconv.ParseFloat(v, 64)
				if err != nil {
					http.Error(w, "invalid coordinates", http.StatusBadRequest)
					return
				}
				forecast.Geometry.Coordinates = append(forecast.Geometry.Coordinates, cord)
			}

			body, err := json.Marshal(&forecast)
			if err != nil {
				http.Error(w, "failed to marshal forecast", http.StatusInternalServerError)
				return
			}

			_, err = w.Write(body)
			if err != nil {
				http.Error(w, "failed to write response", http.StatusInternalServerError)
				return
			}
		}
	}))

	client = &Client{BaseURL: testServer.URL, HTTPClient: &http.Client{}}

	code := m.Run()
	os.Exit(code)
}

func TestGetCompleteForecast(t *testing.T) {
	var tests = []struct {
		name         string
		giveLat      float64
		giveLon      float64
		giveAltitude float64
		wantForecast Forecast
		wantErr      error
	}{
		{
			name:         "should call getCompleteForecast with valid coordinates",
			giveLat:      1.0,
			giveLon:      2.0,
			giveAltitude: 3.0,
			wantForecast: Forecast{
				Geometry: Geometry{
					Coordinates: []float64{1.0, 2.0, 3.0},
				},
			},
			wantErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			forecast, err := client.GetCompleteForecast(ctx, test.giveLat, test.giveLon, test.giveAltitude)
			if err != nil {
				assert.Equal(t, test.wantErr, err)
			}
			assert.Equal(t, test.wantForecast, forecast)
		})
	}
}
