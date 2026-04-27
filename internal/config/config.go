package config

import "os"

type Currency struct {
	Code        string // ISO 4217, e.g. "INR"
	Symbol      string // e.g. "₹"
	SubunitName string // e.g. "paise"
}

type Config struct {
	DatabaseURL    string
	Port           string
	Currency       Currency
	RunMigrations  bool   // set RUN_MIGRATIONS=true to auto-migrate on startup
	MigrationsPath string // path to migrations dir; default "./migrations"
	ServeStatic    bool   // set SERVE_STATIC=true to serve frontend/dist from "/"
	StaticDir      string // path to built frontend assets; default "./frontend/dist"
	CORSOrigins    string // comma-separated allowed origins; default "http://localhost:5173"
}

func Load() Config {
	return Config{
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/go_money?sslmode=disable"),
		Port:        getEnv("PORT", "8080"),
		Currency: Currency{
			Code:        getEnv("CURRENCY_CODE", "INR"),
			Symbol:      getEnv("CURRENCY_SYMBOL", "₹"),
			SubunitName: getEnv("CURRENCY_SUBUNIT_NAME", "paise"),
		},
		RunMigrations:  getEnv("RUN_MIGRATIONS", "false") == "true",
		MigrationsPath: getEnv("MIGRATIONS_PATH", "./migrations"),
		ServeStatic:    getEnv("SERVE_STATIC", "false") == "true",
		StaticDir:      getEnv("STATIC_DIR", "./frontend/dist"),
		CORSOrigins:    getEnv("CORS_ORIGINS", "http://localhost:5173"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
