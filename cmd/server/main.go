package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/shivamverma/go-money/internal/account"
	"github.com/shivamverma/go-money/internal/audit"
	"github.com/shivamverma/go-money/internal/config"
	"github.com/shivamverma/go-money/internal/customer"
	"github.com/shivamverma/go-money/internal/db"
	"github.com/shivamverma/go-money/internal/ledger"
	"github.com/shivamverma/go-money/internal/reversal"
	"github.com/shivamverma/go-money/internal/transaction"
)

func main() {
	loadDotEnv()

	cfg := config.Load()

	if cfg.RunMigrations {
		runMigrations(cfg.MigrationsPath, cfg.DatabaseURL)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to db: %v", err)
	}
	defer pool.Close()

	customerStore := customer.NewStore(pool)
	accountStore := account.NewStore(pool)
	txStore := transaction.NewStore(pool)
	ledgerStore := ledger.NewStore()
	auditStore := audit.NewStore(pool)

	txService := transaction.NewService(pool, accountStore, txStore, ledgerStore, auditStore)
	reversalService := reversal.NewService(pool, accountStore, txStore, ledgerStore, auditStore)

	customerHandler := customer.NewHandler(customerStore)
	accountHandler := account.NewHandler(accountStore, cfg.Currency)
	txHandler := transaction.NewHandler(txService, txStore, cfg.Currency)
	reversalHandler := reversal.NewHandler(reversalService, cfg.Currency)
	auditHandler := audit.NewHandler(auditStore, cfg.Currency)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.CORSOrigins))

	r.Route("/api", func(r chi.Router) {
		r.Route("/customers", func(r chi.Router) {
			r.Get("/", customerHandler.List)
			r.Post("/", customerHandler.Create)
			r.Get("/{id}", customerHandler.Get)
			r.Get("/{id}/accounts", accountHandler.ListByCustomer)
		})

		r.Route("/accounts", func(r chi.Router) {
			r.Get("/", accountHandler.List)
			r.Post("/", accountHandler.Create)
			r.Get("/{id}", accountHandler.Get)
		})

		r.Route("/transactions", func(r chi.Router) {
			r.Get("/", txHandler.List)
			r.Post("/", txHandler.Create)
			r.Get("/{id}", txHandler.Get)
			r.Post("/{id}/reverse", reversalHandler.Reverse)
		})

		r.Get("/audit-log", auditHandler.List)
	})

	if cfg.ServeStatic {
		fs := http.FileServer(http.Dir(cfg.StaticDir))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := cfg.StaticDir + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, cfg.StaticDir+"/index.html")
				return
			}
			fs.ServeHTTP(w, r)
		})
		log.Printf("serving static files from %s", cfg.StaticDir)
	}

	addr := ":" + cfg.Port
	log.Printf("server listening on %s (currency: %s %s)", addr, cfg.Currency.Code, cfg.Currency.Symbol)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runMigrations(migrationsPath, databaseURL string) {
	if !strings.HasPrefix(migrationsPath, "file://") {
		migrationsPath = "file://" + migrationsPath
	}
	m, err := migrate.New(migrationsPath, databaseURL)
	if err != nil {
		log.Fatalf("migrations init: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migrations up: %v", err)
	}
	log.Println("migrations applied")
}

func corsMiddleware(origins string) func(http.Handler) http.Handler {
	allowed := strings.Split(origins, ",")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			for _, o := range allowed {
				if strings.TrimSpace(o) == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range splitLines(string(data)) {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		for i, c := range line {
			if c == '=' {
				key := line[:i]
				val := line[i+1:]
				for j, ch := range val {
					if ch == '#' {
						val = val[:j]
						break
					}
				}
				if os.Getenv(key) == "" {
					os.Setenv(key, trimSpace(val))
				}
				break
			}
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, trimSpace(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, trimSpace(s[start:]))
	}
	return lines
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
