package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/fitbit"
)

func main() {
	conf := &OauthConfig{
		State: uuid.NewString(),
		Config: &oauth2.Config{
			ClientID:     os.Getenv("OAUTH2_CLIENT_ID"),
			ClientSecret: os.Getenv("OAUTH2_CLIENT_SECRET"),
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
		},
		AuthCode: os.Getenv("OAUTH2_AUTH_CODE"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth", oauthHandler(conf))
	mux.HandleFunc("/authorize", authorizeHandler(conf))
	mux.HandleFunc("/oauth-redirect", oauthRedirectHandler(conf))
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})
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

func handler(config *OauthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !config.IsAuthorized() {
			http.Redirect(w, r, "/auth", http.StatusTemporaryRedirect)
			return
		}
		tok, err := config.Exchange(r.Context(), config.AuthCode, oauth2.AccessTypeOffline)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error exchanging token: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Token: %+#v\n", tok)
		client := config.Client(r.Context(), tok)

		res, err := client.PostForm("https://api.fitbit.com/1.1/oauth2/introspect", url.Values{"token": []string{tok.AccessToken}})
		if err != nil {
			log.Printf("Error introspecting token: %v", err)
		}
		if err := printResponse(res); err != nil {
			log.Printf("Error reading token introspect response: %v", err)
		}
		res, err = client.Get("https://api.fitbit.com/1/user/-/profile.json")
		if err != nil {
			log.Printf("Error getting devices: %v", err)
		}
		if err := printResponse(res); err != nil {
			log.Printf("Error reading devices: %v", err)
		}
		fmt.Fprint(w, `<!DOCTYPE html>
        <html>
            <body>
                <h1>Fitbit exporter</h1>
                <p>You successfully authorized this exporter to fetch your data from Fitbit</p>
                <p>Visit the metrics endpoint at <a href="/metrics">/metrics</a></p>
            </body>
        </html>
        `)
	}
}

func oauthHandler(config *OauthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := authTemplate.Execute(w, "auth.tpl.html"); err != nil {
			http.Error(w, fmt.Sprintf("error while executing template: %v", err), http.StatusInternalServerError)
		}
	}
}

func authorizeHandler(config *OauthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("error parsing form: %v", err), http.StatusBadRequest)
		}
		redirectURL, err := url.Parse(r.Form.Get("redirectURL"))
		if err != nil {
			log.Printf("Error parsing redirect URL: %v", err)
		} else {
			redirectURL.Path = "/oauth-redirect"
			config.RedirectURL = redirectURL.String()
		}
		fitbitURL := config.AuthCodeURL(config.State, oauth2.AccessTypeOffline)
		http.Redirect(w, r, fitbitURL, http.StatusTemporaryRedirect)
	}
}

func oauthRedirectHandler(config *OauthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		authCode := params.Get("code")
		state := params.Get("state")
		if !config.IsStateValid(state) {
			http.Error(w, "State does not match", http.StatusBadRequest)
			return
		}
		config.AuthCode = authCode
		templateValues := map[string]interface{}{
			"scopes": config.Scopes,
		}
		if err := oauthRedirectTemplate.ExecuteTemplate(w, "oauth-redirect.tpl.html", templateValues); err != nil {
			http.Error(w, fmt.Sprintf("error while executing template: %v", err), http.StatusInternalServerError)
		}
	}
}

var authTemplate = template.Must(template.ParseFiles("./auth.tpl.html"))
var oauthRedirectTemplate = template.Must(template.ParseFiles("./oauth-redirect.tpl.html"))

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

type OauthConfig struct {
	*oauth2.Config
	AuthCode string
	State    string
	sync.Mutex
}

func (o OauthConfig) IsAuthorized() bool {
	o.Lock()
	defer o.Unlock()
	return o.AuthCode != ""
}

func (o OauthConfig) IsStateValid(state string) bool {
	return o.State == state
}
