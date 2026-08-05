package siat

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	wsdlURL    string
	endpoint   string
	httpClient *http.Client
	headers    map[string]string
	logger     *slog.Logger
}

const siatNamespace = "https://siat.impuestos.gob.bo/"

type Option func(*Client)

func WithLogger(logger *slog.Logger) Option {
	return func(client *Client) {
		if logger != nil {
			client.logger = logger
		}
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func WithHeader(key, value string) Option {
	return func(client *Client) {
		if client.headers == nil {
			client.headers = map[string]string{}
		}
		client.headers[key] = value
	}
}

func WithHeaders(headers map[string]string) Option {
	return func(client *Client) {
		if client.headers == nil {
			client.headers = map[string]string{}
		}
		for key, value := range headers {
			client.headers[key] = value
		}
	}
}

func NewClient(cfg Config, opts ...Option) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client := &Client{
		wsdlURL:  cfg.WSDLURL,
		endpoint: cfg.EffectiveEndpoint(),
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		headers: cfg.CloneHeaders(),
		logger:  slog.Default(),
	}

	for _, option := range opts {
		option(client)
	}

	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	if client.logger == nil {
		client.logger = slog.Default()
	}
	if client.headers == nil {
		client.headers = map[string]string{}
	}

	return client, nil
}

func (c *Client) Do(ctx context.Context, operation string, requestEnvelope any) ([]byte, error) {
	payload, err := xml.Marshal(requestEnvelope)
	if err != nil {
		return nil, fmt.Errorf("siat %s: marshal request: %w", operation, err)
	}

	if c.logger != nil {
		c.logger.Debug("siat soap request", "operation", operation, "endpoint", c.endpoint, "payload", string(payload))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("siat %s: create request: %w", operation, err)
	}

	req.Header.Set("Content-Type", soapContentType)
	req.Header.Set("Accept", soapContentType)
	req.Header.Set("User-Agent", "supay-siat-go/1.0")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("siat %s: execute request: %w", operation, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("siat %s: read response: %w", operation, err)
	}

	if c.logger != nil {
		c.logger.Debug("siat soap response", "operation", operation, "status", resp.StatusCode, "elapsed", time.Since(start).String(), "payload", string(body))
	}

	if resp.StatusCode >= 400 {
		if fault, parseErr := parseSOAPFault(body); parseErr == nil && fault != nil {
			return nil, &SOAPFaultError{Operation: operation, Fault: *fault}
		}
		return nil, fmt.Errorf("siat %s: unexpected http status %d", operation, resp.StatusCode)
	}

	if fault, parseErr := parseSOAPFault(body); parseErr == nil && fault != nil {
		return nil, &SOAPFaultError{Operation: operation, Fault: *fault}
	} else if parseErr != nil {
		return nil, fmt.Errorf("siat %s: decode soap fault: %w", operation, parseErr)
	}

	return body, nil
}

// DoRaw sends a pre-built XML payload as the SOAP body without additional marshaling.
func (c *Client) DoRaw(ctx context.Context, operation string, rawPayload []byte) ([]byte, error) {
	if c.logger != nil {
		c.logger.Debug("siat soap request (raw)", "operation", operation, "endpoint", c.endpoint, "payload", string(rawPayload))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(rawPayload))
	if err != nil {
		return nil, fmt.Errorf("siat %s: create request: %w", operation, err)
	}

	req.Header.Set("Content-Type", soapContentType)
	req.Header.Set("Accept", soapContentType)
	req.Header.Set("User-Agent", "supay-siat-go/1.0")
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("siat %s: execute request: %w", operation, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("siat %s: read response: %w", operation, err)
	}

	if c.logger != nil {
		c.logger.Debug("siat soap response (raw)", "operation", operation, "status", resp.StatusCode, "elapsed", time.Since(start).String(), "payload", string(body))
	}

	if resp.StatusCode >= 400 {
		if fault, parseErr := parseSOAPFault(body); parseErr == nil && fault != nil {
			return nil, &SOAPFaultError{Operation: operation, Fault: *fault}
		}
		return nil, fmt.Errorf("siat %s: unexpected http status %d", operation, resp.StatusCode)
	}

	if fault, parseErr := parseSOAPFault(body); parseErr == nil && fault != nil {
		return nil, &SOAPFaultError{Operation: operation, Fault: *fault}
	} else if parseErr != nil {
		return nil, fmt.Errorf("siat %s: decode soap fault: %w", operation, parseErr)
	}

	return body, nil
}

// CloneWithEndpoint returns a shallow copy of the client configured to use a different endpoint.
func (c *Client) CloneWithEndpoint(endpoint string) *Client {
	nc := &Client{
		wsdlURL:    c.wsdlURL,
		endpoint:   endpoint,
		httpClient: c.httpClient,
		headers:    map[string]string{},
		logger:     c.logger,
	}
	for k, v := range c.headers {
		nc.headers[k] = v
	}
	return nc
}

// ServiceEndpoint resuelve la URL del endpoint SOAP para un servicio del SIAT.
// Deriva desde el endpoint base configurado: si este incluye el nombre de un
// servicio conocido, lo reemplaza; si no (p.ej. un endpoint custom en tests o un
// proxy), devuelve el endpoint tal cual.
func (c *Client) ServiceEndpoint(service Service) string {
	base := strings.TrimSuffix(c.endpoint, "?wsdl")
	for _, known := range []Service{ServiceCodigos, ServiceOperaciones, ServiceSincronizacion} {
		suffix := "/" + known.String()
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix) + "/" + service.String()
		}
	}
	return base
}

func (c *Client) Endpoint() string { return c.endpoint }

func (c *Client) WSDLURL() string { return c.wsdlURL }

func (c *Client) Header(key string) string { return c.headers[key] }

func (c *Client) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = map[string]string{}
	}
	c.headers[key] = value
}
