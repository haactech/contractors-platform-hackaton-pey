package application

import (
	"context"
	"contractors-platform/backend/internal/modules/payments/domain"
	"errors"
	"log"
	"strings"
)

// CreatePayment arma el cobro, lo dirige a la cuenta del vendedor y lo guarda si Pey lo acepta.
type CreatePayment struct {
	repository domain.Repository
	vendors    domain.VendorSource
	clients    domain.ClientDirectory
	collector  domain.Collector
	links      domain.Links
}

// NewCreatePayment prepara el acto con el almacén, la cuenta, los clientes y Pey.
func NewCreatePayment(repository domain.Repository, vendors domain.VendorSource, clients domain.ClientDirectory, collector domain.Collector, links domain.Links) *CreatePayment {
	return &CreatePayment{repository: repository, vendors: vendors, clients: clients, collector: collector, links: links}
}

// Create valida el pedido de la pantalla, carga el cliente y la cuenta de ese vendedor y crea el cobro en Pey.
func (act *CreatePayment) Create(ctx context.Context, input domain.Request) (domain.View, map[string]string, error) {
	payment, fieldErrors := domain.Build(input)
	if len(fieldErrors) > 0 {
		return domain.View{}, fieldErrors, nil
	}
	client, fieldErrors, err := act.resolveClient(payment)
	if err != nil || len(fieldErrors) > 0 {
		return domain.View{}, fieldErrors, err
	}
	payment = domain.ApplyClient(payment, client)
	if fieldErrors = domain.DirectPhoneReady(payment); len(fieldErrors) > 0 {
		return domain.View{}, fieldErrors, nil
	}
	account, found, err := act.vendors.Account(payment.VendorID)
	if err != nil {
		return domain.View{}, nil, err
	}
	if !found {
		return domain.View{}, nil, domain.ErrVendorNotFound
	}
	if !domain.AccountReady(account) {
		return domain.View{}, map[string]string{domain.MethodField(): domain.AccountMessage()}, nil
	}
	payment.ID, err = act.repository.Allocate()
	if err != nil {
		return domain.View{}, nil, err
	}
	payment.BankAccountID = account.BankAccountID
	path, body, ok := domain.Encode(payment, act.links)
	if !ok {
		return domain.View{}, nil, errors.New("el cobro no se pudo armar")
	}
	response, err := act.collector.Collect(ctx, path, body)
	if err != nil {
		log.Printf("payments: pey %s: %v", payment.ID, err)
		return domain.View{}, nil, domain.ErrCollector
	}
	result, ok := domain.Decode(payment.Method, response)
	if !ok {
		log.Printf("payments: pey %s: respuesta sin cobro usable", payment.ID)
		return domain.View{}, nil, domain.ErrCollector
	}
	saved, err := act.repository.Save(domain.Attach(payment, result))
	if err != nil {
		return domain.View{}, nil, err
	}
	return domain.Present(saved), nil, nil
}

func (act *CreatePayment) resolveClient(payment domain.Payment) (domain.Client, map[string]string, error) {
	if strings.TrimSpace(payment.ClientID) != "" {
		client, found, err := act.clients.Find(payment.VendorID, payment.ClientID)
		if err != nil {
			return domain.Client{}, nil, err
		}
		if !found {
			return domain.Client{}, map[string]string{domain.ClientIDField(): domain.ClientMissingMessage()}, nil
		}
		return client, nil, nil
	}
	clients, err := act.clients.List(payment.VendorID)
	if err != nil {
		return domain.Client{}, nil, err
	}
	client, ok := domain.MatchClient(clients, payment.ClientName, payment.ClientPhone)
	if !ok {
		return domain.Client{}, map[string]string{domain.ClientField(): domain.ClientMissingMessage()}, nil
	}
	return client, nil, nil
}
