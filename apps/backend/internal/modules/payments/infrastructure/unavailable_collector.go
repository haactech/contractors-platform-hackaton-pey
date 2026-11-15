package infrastructure

import (
	"context"
	"errors"
)

// UnavailableCollector no llama a Pey cuando la llave de la plataforma no está configurada.
type UnavailableCollector struct{}

// Collect informa que Pey no está configurado.
func (UnavailableCollector) Collect(context.Context, string, []byte) ([]byte, error) {
	return nil, errors.New("pey no está configurado")
}
