package http

import (
	"net/http"
)

type AuthHttpDoer interface {
	Get(url string) (*http.Response, error)
}

type AuthHttpClient struct {
	token string
}

func NewAuthHttpClient(token string) *AuthHttpClient {
	return &AuthHttpClient{
		token: token,
	}
}

func (c *AuthHttpClient) Get(url string) (*http.Response, error) {
	var client http.Client

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+c.token)

	if err != nil {
		return nil, err
	}

	return client.Do(req)
}
