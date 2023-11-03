package siegeserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	APIKey string
	Scheme string
	Host   string
}

func NewClient() *Client {
	// TODO should be configurable via env vars somewhere
	//  locally want http://localhost:3000
	//  live want something like https://siegeai.com
	return &Client{
		APIKey: "12345",
		Scheme: "http",
		Host:   "localhost:3000",
	}
}

type ListenerConfig struct {
	ListenerID string `json:"listenerID"`
}

func (c *Client) Startup(ctx context.Context) (*ListenerConfig, error) {
	u := c.formatURL("/api/v1/listener/startup")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, err
	}

	var config ListenerConfig
	if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return &config, nil
}

type ListenerShutdownRequest struct {
	ListenerID string `json:"listenerID"`
}

func (c *Client) Shutdown(ctx context.Context, listenerID string) error {
	u := c.formatURL("/api/v1/listener/shutdown")

	bs, err := json.Marshal(&ListenerShutdownRequest{ListenerID: listenerID})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(bs))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return errors.New("unexpected status code")
	}

	return nil
}

type ListenerUpdate struct {
	ListenerID string   `json:"listenerID"`
	Schemas    []string `json:"schemas"`
	Metrics    string   `json:"metrics"`
}

func (c *Client) Update(ctx context.Context, args ListenerUpdate) error {
	u := c.formatURL("/api/v1/listener/update")

	bs, err := json.Marshal(&args)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(bs))
	if err != nil {
		panic(err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	if err != nil {
		return err
	}

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return errors.New("unexpected status code")
	}

	return nil
}

func (c *Client) formatURL(path string) string {
	u := url.URL{Scheme: c.Scheme, Host: c.Host, Path: path}
	return u.String()
}
