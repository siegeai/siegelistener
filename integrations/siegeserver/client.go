package siegeserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/siegeai/siegelistener/infer"
)

type Client struct {
	APIKey string
	Server string
}

var (
	ErrUnexpectedResponse = errors.New("unexpected response code")
)

func NewClient(apikey, server string) (*Client, error) {
	client := &Client{
		APIKey: apikey,
		Server: server,
	}
	return client, nil
}

type ListenerConfig struct {
	ListenerID string `json:"listenerID"`
}

func (c *Client) Startup(ctx context.Context) (*ListenerConfig, error) {
	//u := c.formatURL("/api/v1/listener/startup")
	//
	//req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	//if err != nil {
	//	return nil, err
	//}
	//req.Header.Add("Content-Type", "application/json")
	//req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	//
	//client := http.Client{}
	//res, err := client.Do(req)
	//if err != nil {
	//	return nil, err
	//}
	//
	//if res.StatusCode != http.StatusOK {
	//	return nil, ErrUnexpectedResponse
	//}
	//
	//var config ListenerConfig
	//if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
	//	return nil, err
	//}
	//defer res.Body.Close()

	config := ListenerConfig{ListenerID: "12345"}
	return &config, nil
}

type ListenerShutdownRequest struct {
	ListenerID string `json:"listenerID"`
}

func (c *Client) Shutdown(ctx context.Context, listenerID string) error {
	//u := c.formatURL("/api/v1/listener/shutdown")
	//
	//bs, err := json.Marshal(&ListenerShutdownRequest{ListenerID: listenerID})
	//if err != nil {
	//	return err
	//}
	//
	//req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(bs))
	//if err != nil {
	//	panic(err)
	//}
	//req.Header.Add("Content-Type", "application/json")
	//req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	//
	//client := http.Client{}
	//res, err := client.Do(req)
	//
	//if err != nil {
	//	return err
	//}
	//
	//if res.StatusCode != http.StatusOK {
	//	return ErrUnexpectedResponse
	//}

	return nil
}

type ListenerUpdate struct {
	ListenerID string            `json:"listenerID"`
	Schemas    []string          `json:"schemas"`
	Metrics    string            `json:"metrics"`
	Events     []*infer.EventLog `json:"events"`
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
		return ErrUnexpectedResponse
	}

	return nil
}

func (c *Client) formatURL(path string) string {
	return fmt.Sprintf("%s%s", c.Server, path)
}
