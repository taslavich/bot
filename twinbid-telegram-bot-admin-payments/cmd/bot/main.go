package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"twinbid-telegram-bot/internal/backend"
	"twinbid-telegram-bot/internal/config"
	"twinbid-telegram-bot/internal/server"
	"twinbid-telegram-bot/internal/tgbot"
	"twinbid-telegram-bot/internal/tokenstore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tokens := tokenstore.NewJSONStore(cfg.TokenStorePath)
	backendClient := backend.NewClient(cfg, tokens)
	if err := backendClient.EnsureLoggedIn(ctx); err != nil {
		log.Fatalf("backend login error: %v", err)
	}

	modes, err := tgbot.NewModeStore(cfg.ChatStorePath)
	if err != nil {
		log.Fatalf("mode store error: %v", err)
	}

	bot, err := tgbot.New(cfg, backendClient, modes)
	if err != nil {
		log.Fatalf("telegram bot init error: %v", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.New(cfg, bot).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("http server listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if err := bot.StartPolling(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("telegram polling error: %v", err)
	}
}
