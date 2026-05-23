package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/gallowaysoftware/toqui/backend/internal/config"
)

func main() {
	direction := flag.String("direction", "up", "Migration direction: up or down")
	steps := flag.Int("steps", 0, "Number of steps (0 = all)")
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://toqui:toqui@localhost:5432/toqui?sslmode=disable"
	}

	// Resolve gcsm:// secret references (e.g. gcsm://staging-database-url).
	if strings.HasPrefix(databaseURL, "gcsm://") {
		projectID := os.Getenv("FIRESTORE_PROJECT_ID")
		if projectID == "" {
			// Fall back to GCP project from Cloud Run metadata.
			projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		}
		resolved, err := config.ResolveSecretValue(databaseURL, projectID)
		if err != nil {
			log.Fatalf("resolve DATABASE_URL secret: %v", err)
		}
		databaseURL = resolved
		log.Println("resolved DATABASE_URL from GCP Secret Manager")
	}

	// Docker image has migrations at /migrations; local dev at db/migrations.
	migrationsPath := "file://db/migrations"
	if _, err := os.Stat("/migrations"); err == nil {
		migrationsPath = "file:///migrations"
	}

	m, err := migrate.New(migrationsPath, databaseURL)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	defer m.Close()

	switch *direction {
	case "up":
		if *steps > 0 {
			err = m.Steps(*steps)
		} else {
			err = m.Up()
		}
	case "down":
		if *steps > 0 {
			err = m.Steps(-*steps)
		} else {
			err = m.Down()
		}
	default:
		log.Fatalf("unknown direction: %s", *direction)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migration failed: %v", err)
	}

	fmt.Printf("Migration %s completed successfully\n", *direction)
}
