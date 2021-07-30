package http

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
)

func LogTransport(transport http.RoundTripper) http.RoundTripper {
	return &logTransport{transport: transport}
}

type logTransport struct {
	transport http.RoundTripper
}

func (l *logTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var buf bytes.Buffer
	request.Body = io.NopCloser((io.TeeReader(request.Body, &buf)))
	req, err := httputil.DumpRequest(request, true)
	if err != nil {
		log.Printf("Error dumping request: %v", err)
	}
	log.Println(string(req))
	response, respErr := l.transport.RoundTrip(request)
	if respErr != nil {
		log.Printf("Error sending request: %v", respErr)
	}
	buf.Reset()
	response.Body = io.NopCloser(io.TeeReader(response.Body, &buf))
	res, err := httputil.DumpResponse(response, true)
	if err != nil {
		log.Printf("Error dumping request: %v", err)
	}
	log.Println(string(res))
	return response, respErr
}
