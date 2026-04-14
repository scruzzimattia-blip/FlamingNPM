package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"github.com/flamingnpm/waf/internal/api"
	"github.com/flamingnpm/waf/internal/database"
	"github.com/flamingnpm/waf/internal/models"
	"github.com/flamingnpm/waf/internal/proxy"
	"github.com/flamingnpm/waf/internal/waf"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("FlamingNPM WAF wird gestartet...")

	backendURL := getEnv("BACKEND_URL", "http://nginx-proxy-manager:81")
	listenAddr := getEnv("LISTEN_ADDR", ":8080")
	apiAddr := getEnv("API_ADDR", ":8443")
	dbPath := getEnv("DB_PATH", "/data/waf.db")
	maxBodyStr := getEnv("MAX_BODY_SIZE", "1048576")
	rateLimitMaxStr := getEnv("RATE_LIMIT_MAX", "100")
	rateLimitWindowStr := getEnv("RATE_LIMIT_WINDOW", "60")

	maxBody := parseInt64(maxBodyStr, 1048576)
	rateLimitMax := parseInt(rateLimitMaxStr, 100)
	rateLimitWindow := parseInt(rateLimitWindowStr, 60)

	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Datenbank initialisieren fehlgeschlagen: %v", err)
	}
	defer db.Close()
	log.Println("SQLite-Datenbank initialisiert")

	engine, err := waf.NewEngine(db, waf.Config{
		MaxBodySize:     maxBody,
		RateLimitMax:    rateLimitMax,
		RateLimitWindow: rateLimitWindow,
	})
	if err != nil {
		log.Fatalf("WAF-Engine initialisieren fehlgeschlagen: %v", err)
	}
	log.Println("WAF-Engine gestartet")

	hub := api.NewHub()

	engine.SetOnBlock(func(blocked *models.BlockedRequest) {
		hub.Broadcast(models.WSMessage{
			Type: "blocked_request",
			Data: blocked,
		})
	})

	reverseProxy, err := proxy.New(backendURL, engine)
	if err != nil {
		log.Fatalf("Reverse-Proxy initialisieren fehlgeschlagen: %v", err)
	}
	log.Printf("Reverse-Proxy leitet weiter an: %s", backendURL)

	apiRouter := mux.NewRouter()
	handler := api.NewHandler(db, engine, hub)
	handler.RegisterRoutes(apiRouter)

	apiRouter.PathPrefix("/").Handler(http.FileServer(http.Dir("/app/web/dist")))

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(apiRouter)

	proxyServer := &http.Server{
		Addr:         listenAddr,
		Handler:      reverseProxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	apiServer := &http.Server{
		Addr:         apiAddr,
		Handler:      corsHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := db.CleanExpiredBlocks(); err != nil {
				log.Printf("Abgelaufene Sperren bereinigen fehlgeschlagen: %v", err)
			}
		}
	}()

	go func() {
		log.Printf("WAF-Proxy lauscht auf %s", listenAddr)
		if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Proxy-Server Fehler: %v", err)
		}
	}()

	go func() {
		log.Printf("Dashboard-API lauscht auf %s", apiAddr)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API-Server Fehler: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Server wird heruntergefahren...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	proxyServer.Shutdown(ctx)
	apiServer.Shutdown(ctx)

	log.Println("Server erfolgreich beendet")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseInt(s string, fallback int) int {
	v := 0
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parseInt64(s string, fallback int64) int64 {
	v := int64(0)
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
