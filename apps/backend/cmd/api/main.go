package main

import (
	"context"
	"contractors-platform/backend/internal/app"
	clientsinfra "contractors-platform/backend/internal/modules/clients/infrastructure"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"contractors-platform/backend/internal/modules/onboarding/infrastructure"
	paymentsdomain "contractors-platform/backend/internal/modules/payments/domain"
	paymentsinfra "contractors-platform/backend/internal/modules/payments/infrastructure"
	"contractors-platform/backend/internal/platform/database"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func main() {
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		log.Fatal("PORT must be an integer between 1 and 65535")
	}
	var banks domain.BankRegistrar = infrastructure.NewUnavailablePey()
	var collector paymentsdomain.Collector = paymentsinfra.UnavailableCollector{}
	if key := os.Getenv("PEY_API_KEY"); key != "" {
		peyURL := os.Getenv("PEY_API_URL")
		if peyURL == "" {
			peyURL = "https://api.pey.mx"
		}
		client := infrastructure.NewPeyClient(peyURL, key)
		banks = client
		collector = client
	}
	handler := app.NewWith(infrastructure.NewMemoryRepository(), banks)
	// DATABASE_PATH is the vendor contract database. Clients and payments live beside it unless their paths are set.
	if path := os.Getenv("DATABASE_PATH"); path != "" {
		vendors, err := database.OpenSQLite(path)
		if err != nil {
			log.Fatal(err)
		}
		defer vendors.Close()
		clientsPath := os.Getenv("CLIENTS_DATABASE_PATH")
		if clientsPath == "" {
			clientsPath = filepath.Join(filepath.Dir(path), "clients.db")
		}
		clients, err := database.OpenSQLite(clientsPath)
		if err != nil {
			log.Fatal(err)
		}
		defer clients.Close()
		repository, err := infrastructure.NewSQLiteRepository(vendors)
		if err != nil {
			log.Fatal(err)
		}
		clientsRepository, err := clientsinfra.NewSQLiteRepository(clients)
		if err != nil {
			log.Fatal(err)
		}
		paymentsPath := os.Getenv("PAYMENTS_DATABASE_PATH")
		if paymentsPath == "" {
			paymentsPath = filepath.Join(filepath.Dir(path), "payments.db")
		}
		paymentsDB, err := database.OpenSQLite(paymentsPath)
		if err != nil {
			log.Fatal(err)
		}
		defer paymentsDB.Close()
		paymentsRepository, err := paymentsinfra.NewSQLiteRepository(paymentsDB)
		if err != nil {
			log.Fatal(err)
		}
		handler = app.Mount(repository, banks, paymentsRepository, paymentsinfra.NewSQLiteVendors(vendors), collector, paymentsdomain.Links{
			Success: os.Getenv("PAYMENT_SUCCESS_URL"),
			Back:    os.Getenv("PAYMENT_BACK_URL"),
		}, clientsRepository, clientsinfra.NewSQLiteVendors(vendors))
		log.Printf("Using SQLite vendors at %s, clients at %s, payments at %s", path, clientsPath, paymentsPath)
	}
	server := &http.Server{Addr: net.JoinHostPort(host, port), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Print(err)
		}
	}()
	log.Printf("API listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
