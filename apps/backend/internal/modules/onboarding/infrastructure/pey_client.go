package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PeyClient registra una CLABE en la API de Pey.
type PeyClient struct {
	baseURL string
	key     string
	http    *http.Client
}

// NewPeyClient prepara el cliente con la URL y la llave de la plataforma.
func NewPeyClient(baseURL, key string) *PeyClient {
	return &PeyClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    &http.Client{Timeout: 8 * time.Second},
	}
}

// Register envía el alta de la CLABE y devuelve la respuesta de Pey.
func (client *PeyClient) Register(ctx context.Context, body []byte) ([]byte, error) {
	return client.Collect(ctx, "/bank-account", body)
}

// Lookup consulta el estado vigente en Pey de una cuenta bancaria ya registrada.
func (client *PeyClient) Lookup(ctx context.Context, bankAccountID string) ([]byte, error) {
	id := strings.TrimSpace(bankAccountID)
	if id == "" {
		return nil, errors.New("bankAccountID requerido")
	}
	return client.do(ctx, http.MethodGet, "/bank-account/"+url.PathEscape(id), nil)
}

// Collect envía un cuerpo ya armado a una ruta de Pey y devuelve la respuesta.
func (client *PeyClient) Collect(ctx context.Context, path string, body []byte) ([]byte, error) {
	return client.do(ctx, http.MethodPost, path, body)
}

func (client *PeyClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.key)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("pey respondió %d", response.StatusCode)
	}
	return payload, nil
}
