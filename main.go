package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/fitbit"
)

func main() {
	conf := &oauth2.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
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
		Endpoint: fitbit.Endpoint,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth", oauthHandler(conf))
	mux.HandleFunc("/authorize", oauthHandler(conf))
	mux.HandleFunc("/", handler(conf))
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
		cancel()
		done <- true
	}()

	<-done
	fmt.Println("exiting")

}

func handler(config *oauth2.Config) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		// Redirect user to consent page to ask for permission
		// for the scopes specified above.
		url := config.AuthCodeURL("state", oauth2.AccessTypeOffline)
		fmt.Printf("Visit the URL for the auth dialog: %v", url)
		// tok, err := config.Token(ctx)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// log.Printf("Token: %+#v\n", tok)
		// client := oauth2.NewClient(ctx, config.TokenSource(ctx))
		// client := oauth2.NewClient(r.Context())

		// res, err := client.PostForm("https://api.fitbit.com/1.1/oauth2/introspect", url.Values{"token": []string{tok.AccessToken}})
		// if err != nil {
		// 	log.Printf("Error introspecting token: %v", err)
		// 	os.Exit(1)
		// }
		// if err := printResponse(res); err != nil {
		// 	log.Printf("Error reading token introspect response: %v", err)
		// 	os.Exit(1)
		// }
		// res, err := client.Get("https://api.fitbit.com/1/user/-/profile.json")
		// if err != nil {
		// 	log.Printf("Error getting devices: %v", err)
		// 	os.Exit(1)
		// }
		// if err := printResponse(res); err != nil {
		// 	log.Printf("Error reading devices: %v", err)
		// 	os.Exit(1)
		// }
	}
}

func oauthHandler(config *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := authTemplate.Execute(w, "auth.tpl.html"); err != nil {
			http.Error(w, fmt.Sprintf("error while executing template: %v", err), http.StatusInternalServerError)
		}
	}
}

func authorizeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("error parsing form: %v", err), http.StatusBadRequest)
		}

	}
}

var authTemplate = template.Must(template.ParseFiles("./auth.tpl.html"))

type logTransport struct {
	transport http.RoundTripper
}

func (l *logTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	req, err := httputil.DumpRequest(request, true)
	if err != nil {
		log.Printf("Error dumping request: %v", err)
	}
	log.Println(string(req))
	response, err := l.transport.RoundTrip(request)
	res, err := httputil.DumpResponse(response, true)
	if err != nil {
		log.Printf("Error dumping request: %v", err)
	}
	log.Println(string(res))
	return response, err
}

func printResponse(response *http.Response) error {
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "error reading body")
	}
	log.Printf("Response: \n%s\n", string(body))
	return nil
}
