package http

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestAuthHTTPRejectsMalformedURL(t *testing.T) {
	client := NewAuthHttpClient("test-token")
	response, err := client.Get("://bad-url")
	if err == nil || response != nil {
		t.Fatalf("Get malformed URL = (%v, %v), want nil response and an error", response, err)
	}
}

func TestAuthHTTPBoundsRequestsByDefault(t *testing.T) {
	client := NewAuthHttpClient("test-token")
	client.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			return nil, errors.New("request has no deadline within 5 seconds")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("bounded")), Header: make(http.Header)}, nil
	})
	response, err := client.Get("http://example.invalid/test")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "bounded" {
		t.Fatalf("response = %q, %v, want bounded", body, err)
	}
}

func TestAuthHTTPTimeoutCoversHeadersAndBody(t *testing.T) {
	for _, flushHeaders := range []bool{false, true} {
		name := "headers"
		if flushHeaders {
			name = "body"
		}
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if flushHeaders {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			t.Cleanup(server.Close)

			client := NewAuthHttpClient("test-token")
			client.client.Timeout = 50 * time.Millisecond
			response, err := client.Get(server.URL)
			if err == nil {
				defer response.Body.Close()
				_, err = io.ReadAll(response.Body)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("request error = %v, want deadline exceeded", err)
			}
		})
	}
}
