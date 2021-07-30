package main

import (
	"log"
	"net/http"

	"github.com/mitch000001/fitbit-exporter/pkg/http/rate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	inFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "fitbit",
		Name:      "client_in_flight_requests",
		Help:      "A gauge of in-flight requests for the wrapped client.",
	})

	clientRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "fitbit",
			Name:      "client_api_requests_total",
			Help:      "A counter for requests from the wrapped client.",
		},
		[]string{"code", "method"},
	)

	// dnsLatencyVec uses custom buckets based on expected dns durations.
	// It has an instance label "event", which is set in the
	// DNSStart and DNSDonehook functions defined in the
	// InstrumentTrace struct below.
	dnsLatencyVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "fitbit",
			Name:      "dns_duration_seconds",
			Help:      "Trace dns latency histogram.",
			Buckets:   []float64{.005, .01, .025, .05},
		},
		[]string{"event"},
	)

	// tlsLatencyVec uses custom buckets based on expected tls durations.
	// It has an instance label "event", which is set in the
	// TLSHandshakeStart and TLSHandshakeDone hook functions defined in the
	// InstrumentTrace struct below.
	tlsLatencyVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "fitbit",
			Name:      "tls_duration_seconds",
			Help:      "Trace tls latency histogram.",
			Buckets:   []float64{.05, .1, .25, .5},
		},
		[]string{"event"},
	)

	// histVec has no labels, making it a zero-dimensional ObserverVec.
	histVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "fitbit",
			Name:      "request_duration_seconds",
			Help:      "A histogram of request latencies.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{},
	)

	// Define functions for the available httptrace.ClientTrace hook
	// functions that we want to instrument.
	trace = &promhttp.InstrumentTrace{
		DNSStart: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			dnsLatencyVec.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			tlsLatencyVec.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}

	rateLimiterLimitGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "fitbit",
		Name:      "rate_limiter_limit",
		Help:      "A gauge of the max requests allowed by the API rate limit.",
	})

	rateLimiterRemainingGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "fitbit",
		Name:      "rate_limiter_remaining",
		Help:      "A gauge of the remaining requests allowed by the API rate limit.",
	})

	rateLimiterResetsAfterGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "fitbit",
		Name:      "rate_limiter_reset_after_seconds",
		Help:      "A gauge of the seconds after which the rate limit will be reset.",
	})
)

func instrumentTransport(rateLimitHeaderKeys rate.HeaderKeys) func(t http.RoundTripper) http.RoundTripper {
	return func(t http.RoundTripper) http.RoundTripper {
		return promhttp.InstrumentRoundTripperInFlight(
			inFlightGauge,
			promhttp.InstrumentRoundTripperCounter(
				clientRequestCounter,
				promhttp.InstrumentRoundTripperTrace(
					trace,
					promhttp.InstrumentRoundTripperDuration(
						histVec,
						instrumentRoundTripperRateLimitHeader(
							rateLimiterLimitGauge,
							rateLimiterRemainingGauge,
							rateLimiterResetsAfterGauge,
							rateLimitHeaderKeys,
							t,
						),
					),
				),
			),
		)
	}
}

func instrumentRoundTripperRateLimitHeader(limitGauge, remainingGauge, resetAfterSecondsGauge prometheus.Gauge, rateLimitHeaders rate.HeaderKeys, next http.RoundTripper) promhttp.RoundTripperFunc {
	return promhttp.RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		response, err := next.RoundTrip(r)
		limit, lerr := rate.LimitFromHeader(response.Header, rateLimitHeaders)
		if lerr != nil {
			log.Printf("Error getting rate limit: %v", lerr)
			return response, err
		}
		limitGauge.Set(float64(limit.Limit))
		remainingGauge.Set(float64(limit.Remaining))
		resetAfterSecondsGauge.Set(float64(limit.ResetAfterSeconds))
		return response, err
	})
}
