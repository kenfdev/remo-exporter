package http

import (
	"net/http"
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
		client: &http.Client{},
	}
}

func (c *AuthHttpClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+c.token)

	if err != nil {
		return nil, err
	}

	return c.client.Do(req)
}
