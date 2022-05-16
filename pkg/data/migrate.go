package data

import (
	"errors"
	"fmt"
	"log"

	"github.com/devict/job-board/pkg/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func Migrate(c *config.Config) error {
	m, err := migrate.New("file://sql", c.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to migrate.New: %w", err)
	}

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to migrate Up: %w", err)
		}

		log.Println("no new migrations detected, schema is current")
	}
	return nil
}
