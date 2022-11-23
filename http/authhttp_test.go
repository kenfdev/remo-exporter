package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authhttp "github.com/kenfdev/remo-exporter/http"
)

func TestAuthHttpClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		if want, got := "Bearer dummy_token", r.Header.Get("Authorization"); want != got {
			t.Errorf("unexpected authz header want=%s got=%s", want, got)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	c := authhttp.NewAuthHttpClient("dummy_token")
	resp, err := c.Get(ts.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if want, got := http.StatusOK, resp.StatusCode; want != got {
		t.Fatalf("unexpected status code want=%d got=%d", want, got)
	}
}
