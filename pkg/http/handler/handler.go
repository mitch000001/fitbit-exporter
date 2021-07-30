package handler

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"

	"github.com/mitch000001/fitbit-exporter/pkg/http/oauth"
	"golang.org/x/oauth2"
)

// content holds our static web server content.
//go:embed templates/*.html
var templatesFS embed.FS
var temlates = template.Must(
	template.ParseFS(templatesFS, "templates/*.html"),
)

func AuthMiddleware(config *oauth.Config, h http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if !config.IsAuthorized() {
			http.Redirect(rw, r, "/auth", http.StatusTemporaryRedirect)
			return
		}
		h.ServeHTTP(rw, r)
	}
}

func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
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

func OauthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := temlates.ExecuteTemplate(w, "auth.tpl.html", nil); err != nil {
			http.Error(w, fmt.Sprintf("error while executing template: %v", err), http.StatusInternalServerError)
		}
	}
}

func AuthorizeHandler(config *oauth.Config) http.HandlerFunc {
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

func OauthRedirectHandler(config *oauth.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authCode := r.FormValue("code")
		state := r.FormValue("state")
		if !config.IsStateValid(state) {
			http.Error(w, "State does not match", http.StatusBadRequest)
			return
		}
		if err := config.Authorize(r.Context(), authCode); err != nil {
			http.Error(w, "unable to authorize oauth2 client", http.StatusInternalServerError)
			return
		}
		templateValues := map[string]interface{}{
			"scopes": config.Scopes,
		}
		if err := temlates.ExecuteTemplate(w, "oauth-redirect.tpl.html", templateValues); err != nil {
			http.Error(w, fmt.Sprintf("error while executing template: %v", err), http.StatusInternalServerError)
		}
	}
}
