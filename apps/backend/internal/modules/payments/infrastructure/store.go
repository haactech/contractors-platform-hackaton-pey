package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

var errRejected = errors.New("el cobro no cumple el contrato")

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
