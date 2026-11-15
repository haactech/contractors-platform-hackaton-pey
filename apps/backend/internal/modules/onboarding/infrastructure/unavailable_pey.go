package infrastructure

import (
	"context"
	"errors"
)

// UnavailablePey no llama a Pey cuando la llave de la plataforma no está configurada.
type UnavailablePey struct{}

// NewUnavailablePey crea el registro que falla de inmediato.
func NewUnavailablePey() UnavailablePey { return UnavailablePey{} }

// Register informa que Pey no está configurado.
func (UnavailablePey) Register(context.Context, []byte) ([]byte, error) {
	return nil, errors.New("pey no está configurado")
}

// Collect informa que Pey no está configurado.
func (UnavailablePey) Collect(context.Context, string, []byte) ([]byte, error) {
	return nil, errors.New("pey no está configurado")
}
