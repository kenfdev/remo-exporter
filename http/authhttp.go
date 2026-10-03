package http

import (
	"net/http"
	"time"
)

type AuthHttpDoer interface {
	Get(url string) (*http.Response, error)
}

type AuthHttpClient struct {
	token  string
	client *http.Client
}

func NewAuthHttpClient(token string) *AuthHttpClient {
	return &AuthHttpClient{
		token:  token,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *AuthHttpClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+c.token)
	return c.client.Do(req)
}
