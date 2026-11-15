package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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

// Collect envía un cuerpo ya armado a una ruta de Pey y devuelve la respuesta.
func (client *PeyClient) Collect(ctx context.Context, path string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+client.key)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
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
