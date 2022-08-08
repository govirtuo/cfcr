package metrics

import (
	"net/http"
	"time"

	"github.com/govirtuo/cfcr/handlers"
	"github.com/prometheus/client_golang/prometheus"
)

// Server is the metrics server. It contains all the Prometheus metrics
type Server struct {
	Addr         string
	NumOfDomains prometheus.Gauge
	LastUpdated  *prometheus.GaugeVec
}

// Init initialize the metrics server
func Init(addr, port string) *Server {
	s := Server{
		NumOfDomains: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "cfcr_domains_watched_total",
			Help: "Number of domains watched by cfcr.",
		}),
		LastUpdated: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cfcr_last_updated_timestamp",
			Help: "Last time the domain's TXT records have been updated.",
		}, []string{"domain"}),
	}

	s.Addr = addr + ":" + port

	prometheus.MustRegister(
		s.NumOfDomains,
		s.LastUpdated,
	)
	return &s
}

// Start starts the metrics server
func (s Server) Start() error {
	srv := &http.Server{
		Addr:         s.Addr,
		Handler:      handlers.HandleFunc(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Server) SetNumOfDomainsMetric(num int) {
	s.NumOfDomains.Set(float64(num))
}

func (s *Server) SetDomainLastUpdatedMetric(d string) {
	s.LastUpdated.WithLabelValues(d).SetToCurrentTime()
}
