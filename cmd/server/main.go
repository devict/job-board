package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/devict/job-board/pkg/config"
	"github.com/devict/job-board/pkg/data"
	"github.com/devict/job-board/pkg/server"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	log.SetOutput(os.Stderr)

	if err := run(); err != nil {
		log.Fatalln("main failed to run:", err)
	}

	log.Println("sucessful shutdown")
}

func run() error {
	c, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to LoadConfig: %w", err)
	}

	// migrate the db on startup
	if err := data.Migrate(c); err != nil {
		return fmt.Errorf("migrations failed: %w", err)
	}

	// get our database connection
	db, err := sqlx.Open("postgres", c.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to sqlx.Open: %w", err)
	}

	// TODO: what to do with the background job?
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			log.Println("removing old jobs")
			_, err := db.Exec("DELETE FROM jobs WHERE published_at < NOW() - INTERVAL '30 DAYS'")
			if err != nil {
				log.Println(fmt.Errorf("error clearing old jobs: %w", err))
			}

			select {
			case <-ctx.Done():
				log.Println("shutting down old jobs background process")
				return
			case <-ticker.C:
				continue
			}
		}
	}()

	server, err := server.NewServer(c, db)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	serverErrors := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("server listening on port %s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case <-serverErrors:
		return fmt.Errorf("received server error: %w", err)

	case sig := <-shutdown:
		log.Printf("received shutdown signal %q", sig)

		cancel()

		if err := server.Shutdown(context.Background()); err != nil {
			return fmt.Errorf("failed to server.Shutdown: %w", err)
		}
	}

	wg.Wait()

	return nil
}
