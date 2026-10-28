package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // group timezones must resolve even on images without tzdata

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := connectWithRetry(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}

	a := &app{db: pool, cfg: cfg}
	if cfg.BotToken != "" {
		a.bot = newBot(cfg.BotToken, cfg.WebAppURL)
	}
	srv := &http.Server{Addr: cfg.Addr, Handler: newRouter(a), ReadHeaderTimeout: 10 * time.Second}
	if a.bot != nil {
		// Registered after the router exists, so the webhook endpoint is live when Telegram calls it.
		go a.bot.run(ctx, cfg.BotPolling)
		go a.runNotifier(ctx)
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()
	log.Println("listening on", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func connectWithRetry(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 0; i < 20; i++ {
		pool, err := pgxpool.New(ctx, url)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, lastErr
}
