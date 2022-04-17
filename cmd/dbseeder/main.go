package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/devict/job-board/cmd/dbseeder/lorem"
)

func main() {
	if err := run(); err != nil {
		log.Fatalln("main failed to run:", err)
	}

	log.Println("success!")
}

func run() error {
	if os.Getenv("APP_ENV") != "debug" {
		return fmt.Errorf("must not run in environment: %q", os.Getenv("APP_ENV"))
	}

	db, err := sqlx.Open("postgres", fmt.Sprintf("%s%s", os.Getenv("DATABASE_URL"), "?sslmode=disable"))
	if err != nil {
		return fmt.Errorf("failed to sqlx.Open: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to Ping: %w", err)
	}

	type Job struct {
		// ID           string         `db:"id"`
		Position     string         `db:"position"`
		Organization string         `db:"organization"`
		Url          sql.NullString `db:"url"`
		Description  sql.NullString `db:"description"`
		Email        string         `db:"email"`
		// PublishedAt  time.Time      `db:"published_at"`
	}

	for i := 0; i < 50; i++ {
		j := Job{
			Position:     lorem.WordsRange(1, 3),
			Organization: lorem.WordsRange(2, 4),
			Url:          sql.NullString{String: lorem.URL(), Valid: i%20 != 0},
			Description:  sql.NullString{String: lorem.ParagraphsN(3), Valid: i%40 != 0},
			Email:        lorem.Email(),
		}

		q := `
		INSERT INTO jobs
			(
				position,
				organization,
				url,
				description,
				email,
				published_at
			)
		VALUES
			(
				:position,
				:organization,
				:url,
				:description,
				:email,
				NOW() - JUSTIFY_INTERVAL(RANDOM() * INTERVAL '60 days')
			)
		`

		if _, err := db.NamedExec(q, j); err != nil {
			return fmt.Errorf("failed to Exec: %w", err)
		}
	}

	return nil
}
