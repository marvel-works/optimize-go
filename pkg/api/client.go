/*
Copyright 2020 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

// Config exposes the information for configuring an API Client.
type Config interface {
	// Endpoints returns a resolver for the location of the specified endpoint.
	Endpoints() (func(string) *url.URL, error)
	// Authorize returns a transport that applies the authorization defined by this configuration. The
	// supplied context is used for any additional requests necessary to perform authentication. If this
	// configuration does not define any authorization details, the supplied transport may be returned
	// directly.
	Authorize(ctx context.Context, transport http.RoundTripper) (http.RoundTripper, error)
}

// Client is used to handle interactions with the API Server.
type Client interface {
	// URL returns the location of the specified endpoint
	URL(endpoint string) *url.URL
	// Do performs the interaction specified by the HTTP request
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

// NewClient returns a new client for accessing API server; the supplied context is used for authentication/authorization
// requests and the supplied transport (which may be nil in the case of the default transport) is used for all requests made
// to the API server.
func NewClient(ctx context.Context, cfg Config, transport http.RoundTripper) (Client, error) {
	var err error

	hc := &httpClient{}
	hc.client.Timeout = 10 * time.Second

	// Configure the OAuth2 transport
	hc.client.Transport, err = cfg.Authorize(ctx, transport)
	if err != nil {
		return nil, err
	}

	// Configure the API endpoints
	hc.endpoints, err = cfg.Endpoints()
	if err != nil {
		return nil, err
	}

	return hc, nil
}

type httpClient struct {
	client    http.Client
	endpoints func(string) *url.URL
}

// URL resolves an endpoint to a fully qualified URL.
func (c *httpClient) URL(ep string) *url.URL {
	return c.endpoints(ep)
}

// Do executes an HTTP request using this client and the supplied context.
func (c *httpClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	var body []byte
	done := make(chan struct{})
	go func() {
		body, err = ioutil.ReadAll(resp.Body)
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		err = resp.Body.Close()
		if err == nil {
			err = ctx.Err()
		}
	case <-done:
	}

	return resp, body, err
}
