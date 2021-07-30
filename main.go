package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/kr/pretty"
	"github.com/mitch000001/fitbit-exporter/pkg/fitbit"
	"github.com/mitch000001/fitbit-exporter/pkg/http/handler"
	"github.com/mitch000001/fitbit-exporter/pkg/http/oauth"
	"github.com/mitch000001/fitbit-exporter/pkg/http/rate"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/oauth2"
	oauth_fitbit "golang.org/x/oauth2/fitbit"
)

func init() {
	// Register all of the metrics in the standard registry.
	prometheus.MustRegister(
		clientRequestCounter, tlsLatencyVec, dnsLatencyVec, histVec, inFlightGauge,
		rateLimiterLimitGauge, rateLimiterRemainingGauge, rateLimiterResetsAfterGauge,
	)
}

func main() {
	rateLimitHeaderKeys := rate.HeaderKeys{
		LimitKey:       "Fitbit-Rate-Limit-Limit",
		RemainingKey:   "Fitbit-Rate-Limit-Remaining",
		ResetsAfterKey: "Fitbit-Rate-Limit-Reset",
	}
	rl, err := rate.NewFromHeader(rateLimitHeaderKeys)
	if err != nil {
		log.Printf("Error initializing rate limiter: %v", err)
		os.Exit(1)
	}
	tokenCache, err := oauth.NewJSONFileTokenCache(os.Getenv("OAUTH2_TOKEN_FILE"))
	if err != nil {
		log.Printf("Error initializing token cache: %v", err)
		os.Exit(1)
	}
	conf := &oauth.Config{
		State:               uuid.NewString(),
		RateLimiter:         rl,
		InstrumentTransport: instrumentTransport(rateLimitHeaderKeys),
		Config: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("OAUTH2_REDIRECT_URL"),
			Scopes: []string{
				"activity",
				"heartrate",
				"location",
				"nutrition",
				"profile",
				"settings",
				"sleep",
				"social",
				"weight",
			},
			Endpoint: oauth_fitbit.Endpoint,
		},
	}
	if err := conf.SetTokenCache(tokenCache); err != nil {
		log.Printf("Error setting token cache: %v", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth", handler.OauthHandler())
	mux.HandleFunc("/authorize", handler.AuthorizeHandler(conf))
	mux.HandleFunc("/oauth-redirect", handler.OauthRedirectHandler(conf))
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", handler.AuthMiddleware(conf, handler.Handler()))
	server := &http.Server{
		Addr:    ":3000",
		Handler: mux,
	}
	defer server.Close()
	go func() {
		log.Printf("Starting server at %s", server.Addr)
		if err := server.ListenAndServe(); err != nil {
			log.Printf("Error starting listening server: %v", err)
		}
	}()
	cancelFn := startMetricCollector(conf)
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Println()
		log.Printf("Caught signal %v", sig)
		ctx := context.Background()
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := server.Shutdown(timeoutCtx); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
		cancelFn()
		cancel()
		done <- true
	}()

	<-done
	fmt.Println("exiting")

}

func startMetricCollector(clientProvider oauth.ClientProvider) func() {
	cancel := make(chan bool, 1)
	go func(clientProvider oauth.ClientProvider) {
		for {
			ctx := context.Background()
			ctx, cancelFn := context.WithCancel(ctx)
			select {
			case <-cancel:
				cancelFn()
				return
			case <-time.Tick(10 * time.Second):
				scrapeMetrics(ctx, clientProvider)
				cancelFn()
			}
		}
	}(clientProvider)
	return func() {
		cancel <- true
	}
}

func scrapeMetrics(ctx context.Context, clientProvider oauth.ClientProvider) {
	client, err := clientProvider.Client(ctx)
	if err != nil {
		log.Printf("Error getting client: %v\n", err)
		return
	}

	// res, err := client.PostForm("https://api.fitbit.com/1.1/oauth2/introspect", url.Values{"token": []string{tok.AccessToken}})
	// if err != nil {
	// 	log.Printf("Error introspecting token: %v", err)
	// 	return
	// }
	// if err := printResponse(res); err != nil {
	// 	log.Printf("Error reading token introspect response: %v", err)
	// 	return
	// }
	now := time.Now()
	now10Sec := now.Truncate(60 * time.Minute).Format("15:04")
	time := now.Format("15:04")
	res, err := client.Get(fmt.Sprintf("https://api.fitbit.com/1/user/-/activities/heart/date/today/1d/1sec/time/%s/%s.json", now10Sec, time))
	if err != nil {
		log.Printf("Error getting heartrate: %v", err)
	}
	var buf bytes.Buffer
	var heartRates fitbit.HeartRateResult
	if err := json.NewDecoder(io.TeeReader(res.Body, &buf)).Decode(&heartRates); err != nil {
		log.Printf("Error parsing heartrate: %v", err)
	}
	pretty.Printf("Heartrates:\n%# v\n", heartRates)
	// if err := printResponse(&buf); err != nil {
	// 	log.Printf("Error reading profile: %v", err)
	// 	return
	// }
	log.Println("Metrics scraped")
}

func printResponse(body io.Reader) error {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return errors.Wrap(err, "error reading body")
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, bodyBytes, "", "  "); err != nil {
		log.Printf("Response: \n%s\n", string(bodyBytes))
	}
	log.Printf("Response: \n%s\n", buf.String())
	return nil
}
