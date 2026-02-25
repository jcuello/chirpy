package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/jcuello/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var cfg apiConfig = apiConfig{}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		fmt.Printf("Unable to connect to database %v\n", err)
		os.Exit(1)
	}

	dbQueries := database.New(db)

	serveMux := http.ServeMux{}
	server := http.Server{}
	cfg.db = dbQueries
	cfg.jwtSecret = os.Getenv("JWT_SECRET")
	cfg.polkaApiKey = os.Getenv("POLKA_KEY")
	appUrlPrefix := "/app/"
	appFileServerHandler := http.StripPrefix(appUrlPrefix, http.FileServer(http.Dir(".")))

	serveMux.Handle(appUrlPrefix, cfg.middlewareMetricsInc(appFileServerHandler))
	serveMux.HandleFunc("GET /api/healthz", func(resp http.ResponseWriter, request *http.Request) {
		resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp.WriteHeader(http.StatusOK)
		resp.Write([]byte("OK\n"))

	})
	serveMux.HandleFunc("POST /api/chirps", handlePostChirp)
	serveMux.HandleFunc("GET /api/chirps", handleGetChirps)
	serveMux.HandleFunc("GET /api/chirps/{chirpID}", handleGetSingleChirp)
	serveMux.HandleFunc("DELETE /api/chirps/{chirpID}", handleDeleteChirps)

	serveMux.HandleFunc("POST /api/users", handlePostUser)
	serveMux.HandleFunc("PUT /api/users", handlePutChirp)

	serveMux.HandleFunc("POST /api/polka/webhooks", handlePolkaWebhook)

	serveMux.HandleFunc("POST /api/login", handleLogin)
	serveMux.HandleFunc("POST /api/refresh", handleRefresh)
	serveMux.HandleFunc("POST /api/revoke", handleRevoke)

	serveMux.HandleFunc("GET /admin/metrics", cfg.viewMetrics())
	serveMux.HandleFunc("POST /admin/reset", cfg.resetMetrics())

	server.Handler = &serveMux
	server.Addr = ":8080"

	server.ListenAndServe()

}
