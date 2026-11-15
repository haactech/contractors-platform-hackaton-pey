package main

import (
	"context"
	"contractors-platform/backend/internal/app"
	"contractors-platform/backend/internal/modules/accounts"
	accdomain "contractors-platform/backend/internal/modules/accounts/domain"
	accinfra "contractors-platform/backend/internal/modules/accounts/infrastructure"
	"contractors-platform/backend/internal/modules/clients"
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
	policy, warnings := accounts.PolicyFromEnv(os.Getenv)
	log.Printf(
		"auth policy: emailVerification=%v lockout=%v pinResetCode=%v demoAccount=%v exposeCodes=%v",
		policy.RequireEmailVerification, policy.Lockout, policy.PINResetRequiresCode, policy.DemoAccount, policy.ExposeCodes,
	)
	for _, warning := range warnings {
		log.Printf("WARNING: %s", warning)
	}

	mailer := accdomain.Mailer(accounts.NewLogMailer())
	if smtpHost := os.Getenv("SMTP_HOST"); smtpHost != "" {
		mailer = accounts.NewSMTPMailer(
			smtpHost,
			os.Getenv("SMTP_PORT"),
			os.Getenv("SMTP_USERNAME"),
			os.Getenv("SMTP_PASSWORD"),
			os.Getenv("MAIL_FROM"),
		)
		log.Printf("Using SMTP mailer via %s", smtpHost)
	}

	var onbRepo domain.Repository = infrastructure.NewMemoryRepository()
	var accRepo accdomain.Repository = accinfra.NewMemoryRepository()
	var clientsRepo clients.Repository = clientsinfra.NewMemoryRepository()
	var clientVendors clients.Vendors
	var paymentsRepo paymentsdomain.Repository = paymentsinfra.NewMemoryRepository()
	var paymentVendors paymentsdomain.VendorSource

	// DATABASE_PATH is the vendor & accounts database. Clients and payments live beside it unless their paths are set.
	if path := os.Getenv("DATABASE_PATH"); path != "" {
		vendors, err := database.OpenSQLite(path)
		if err != nil {
			log.Fatal(err)
		}
		defer vendors.Close()

		onbSQLite, err := infrastructure.NewSQLiteRepository(vendors)
		if err != nil {
			log.Fatal(err)
		}
		accSQLite, err := accinfra.NewSQLiteRepository(vendors)
		if err != nil {
			log.Fatal(err)
		}

		clientsPath := os.Getenv("CLIENTS_DATABASE_PATH")
		if clientsPath == "" {
			clientsPath = filepath.Join(filepath.Dir(path), "clients.db")
		}
		clientsDB, err := database.OpenSQLite(clientsPath)
		if err != nil {
			log.Fatal(err)
		}
		defer clientsDB.Close()
		clientsSQLite, err := clientsinfra.NewSQLiteRepository(clientsDB)
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
		paymentsSQLite, err := paymentsinfra.NewSQLiteRepository(paymentsDB)
		if err != nil {
			log.Fatal(err)
		}

		onbRepo = onbSQLite
		accRepo = accSQLite
		clientsRepo = clientsSQLite
		clientVendors = clientsinfra.NewSQLiteVendors(vendors)
		paymentsRepo = paymentsSQLite
		paymentVendors = paymentsinfra.NewSQLiteVendors(vendors)
		log.Printf("Using SQLite vendors at %s, clients at %s, payments at %s", path, clientsPath, paymentsPath)
	}

	handler, err := app.NewWith(app.Dependencies{
		Onboardings:    onbRepo,
		Accounts:       accRepo,
		Clients:        clientsRepo,
		ClientVendors:  clientVendors,
		Payments:       paymentsRepo,
		PaymentVendors: paymentVendors,
		Collector:      collector,
		Links: paymentsdomain.Links{
			Success: os.Getenv("PAYMENT_SUCCESS_URL"),
			Back:    os.Getenv("PAYMENT_BACK_URL"),
		},
		Banks:  banks,
		Hasher: accounts.NewPBKDF2Hasher(0),
		Mailer: mailer,
		Policy: policy,
		Clock:  time.Now,
	})
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              net.JoinHostPort(host, port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
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
