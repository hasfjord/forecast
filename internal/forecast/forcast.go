package forecast

import (
	"context"
	"net/http"

	"github.com/hasfjord/forecast/internal/yr"
)

type ForecastClient interface {
	GetCompleteForecast(ctx context.Context, lat, lon, altitude float64) (yr.Forecast, error)
}

type InfluxClient interface {
	WriteForecast(ctx context.Context, forecast yr.Forecast) error
}

type Position struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
}

type Server struct {
	client   ForecastClient
	db       InfluxClient
	position Position
}

func NewServer(client ForecastClient, db InfluxClient, cfg Config) *Server {
	return &Server{
		client: client,
		db:     db,
		position: Position{
			Latitude:  cfg.PositionLatitude,
			Longitude: cfg.PositionLongitude,
			Altitude:  cfg.PositionAltitude,
		},
	}
}

type Config struct {
	PositionLatitude  float64 `envconfig:"POSITION_LATITUDE" required:"true"`
	PositionLongitude float64 `envconfig:"POSITION_LONGITUDE" required:"true"`
	PositionAltitude  float64 `envconfig:"POSITION_ALTITUDE" required:"true"`
}

func (s *Server) MakeRunForecastHandler() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		err := s.runForecast(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) runForecast(ctx context.Context) error {
	forecast, err := s.client.GetCompleteForecast(ctx, s.position.Latitude, s.position.Longitude, s.position.Altitude)
	if err != nil {
		return err
	}

	err = s.db.WriteForecast(ctx, forecast)
	if err != nil {
		return err
	}
	return nil
}
