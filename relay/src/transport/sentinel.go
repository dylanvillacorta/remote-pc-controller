package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"remote-pc-controller/relay/src/command"
	"time"
)

type SentinelClient struct {
	URL    string
	Client *http.Client
}

func (c SentinelClient) Deliver(ctx context.Context, value command.Command) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("sentinel returned HTTP %d", response.StatusCode)
	}
	return nil
}
func NewClient(url string) SentinelClient {
	return SentinelClient{URL: url, Client: &http.Client{Timeout: 15 * time.Second}}
}
