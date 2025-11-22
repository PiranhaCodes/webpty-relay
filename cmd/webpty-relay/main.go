// Package main provides the entry point for the WebPTY Relay service.
package main

import (
	"log"
	"os"

	"github.com/piranhaCodes/webpty-relay/internal/config"
	"github.com/piranhaCodes/webpty-relay/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("WebPTY Relay starting...")
	log.Printf("Configuration loaded:")
	log.Printf("  Relay Port: %d", cfg.RelayPort)
	log.Printf("  PTY Socket: %s", cfg.PTYSocket)
	log.Printf("  Admin Password: %s", maskPassword(cfg.AdminPassword))
	log.Printf("  JWT Secret: %s", maskSecret(cfg.JWTSecret))

	srv := server.NewServer(cfg)

	if err := srv.Run(); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}

	log.Println("Server stopped")
}

func maskPassword(pwd string) string {
	if pwd == "" {
		return "<not set>"
	}
	if len(pwd) <= 4 {
		return "****"
	}
	return pwd[:2] + "****"
}

func maskSecret(secret string) string {
	if secret == "" {
		return "<not set>"
	}
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****"
}
