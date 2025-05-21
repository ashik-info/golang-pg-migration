package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cmd := flag.String("cmd", "up", "command: up, down, force, steps, version, seed, init, create")
	steps := flag.Int("steps", 0, "steps for 'steps' command")
	force := flag.Int("force", 0, "force schema version")
	env := flag.String("env", "dev", "environment: dev, test, prod")
	seedFile := flag.String("seed-file", "", "optional path to custom seed SQL file")
	migrationName := flag.String("name", "", "name for creating or applying specific migration file")
	flag.Parse()

	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	db := os.Getenv("POSTGRES_DB")

	if user == "" || pass == "" || host == "" || port == "" || db == "" {
		log.Fatal("Missing required env vars")
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, db)

	if *cmd == "init" {
		migrationsDir := filepath.Join("database", "migrations")
		err := createDatabase(user, pass, host, port, db)
		if err != nil {
			log.Fatalf("Failed to create database: %v", err)
		}
		log.Println("✅ Database created if not exists.")
		files, err := os.ReadDir(migrationsDir)
		if err != nil || len(files) == 0 {
			ts := time.Now().Format("20060102_150405")
			base := fmt.Sprintf("%s_init_%s_db", ts, db)
			upPath := filepath.Join(migrationsDir, base+".up.sql")
			downPath := filepath.Join(migrationsDir, base+".down.sql")

			upSQL := `-- Table structure for table app settings
				CREATE TABLE "app_settings" (
					"id" BIGSERIAL NOT NULL,
					"values" JSONB NOT NULL DEFAULT '{}'::jsonb,
					"created_at" TIMESTAMP(0),
					"updated_at" TIMESTAMP(0),
					CONSTRAINT "settings_pkey" PRIMARY KEY ("id")
				);
			`
			downSQL := `DROP TABLE IF EXISTS "app_settings";`

			os.MkdirAll(migrationsDir, 0755)
			os.WriteFile(upPath, []byte(upSQL), 0644)
			os.WriteFile(downPath, []byte(downSQL), 0644)
			log.Printf("✅ Auto-created initial migration files:\n- %s\n- %s\n", upPath, downPath)
		}
		m, err := migrate.New("file://database/migrations", dbURL)
		if err != nil {
			log.Fatalf("Failed to create migrator: %v", err)
		}

		err = m.Up()
		if err != nil && err.Error() != "no change" {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("✅ Migration applied.")

		err = seedDatabase(dbURL, *env, *seedFile)
		if err != nil {
			log.Fatalf("Seeding error: %v", err)
		}
		log.Println("✅ Seed data inserted.")
		return
	}

	if *cmd == "seed" {
		err := seedDatabase(dbURL, *env, *seedFile)
		if err != nil {
			log.Fatalf("Seeding error: %v", err)
		}
		log.Println("✅ Seed data inserted.")
		return
	}

	if *cmd == "create" {
		if *migrationName == "" {
			log.Fatal("Missing migration name for 'create' command")
		}
		timestamp := time.Now().Format("20060102_150405")
		base := fmt.Sprintf("%s_%s", timestamp, *migrationName)
		upPath := filepath.Join("database", "migrations", base+".up.sql")
		downPath := filepath.Join("database", "migrations", base+".down.sql")

		upTemplate := "-- Write your UP migration here\n\n-- Example:\n-- CREATE TABLE example (id SERIAL PRIMARY KEY, name TEXT);\n"
		downTemplate := "-- Write your DOWN migration here\n\n-- Example:\n-- DROP TABLE example;\n"

		os.WriteFile(upPath, []byte(upTemplate), 0644)
		os.WriteFile(downPath, []byte(downTemplate), 0644)
		log.Printf("✅ Created migration files:\n- %s\n- %s\n", upPath, downPath)

		return
	}
	if *cmd == "up" && *migrationName != "" {
		err := applySingleMigration(dbURL, *migrationName, true)
		if err != nil {
			log.Fatalf("Failed to apply migration '%s': %v", *migrationName, err)
		}
		log.Printf("✅ Applied migration: %s.up.sql", *migrationName)
		return
	}
	m, err := migrate.New("file://database/migrations", dbURL)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}

	switch *cmd {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "steps":
		err = m.Steps(*steps)
	case "force":
		err = m.Force(*force)
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("Failed to get version: %v", err)
		}
		fmt.Printf("Current version: %d (dirty: %v)\n", v, dirty)
		return
	default:
		log.Fatalf("Unknown command: %s", *cmd)
	}

	if err != nil && err.Error() != "no change" {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Printf("Migration command '%s' completed.\n", *cmd)
}

func seedDatabase(dbURL, env, file string) error {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	var seedSQL string

	if file != "" {
		bytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		seedSQL = string(bytes)
	} else {
		path := filepath.Join("database", "seeds", fmt.Sprintf("%s.sql", env))
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		seedSQL = string(bytes)
	}

	stmts := strings.Split(seedSQL, ";")
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			log.Printf("Executing seed statement: %s", stmt)
			_, err := db.Exec(stmt)
			if err != nil {
				return fmt.Errorf("statement failed: %s\n%w", stmt, err)
			}
		}
	}

	return nil
}

func applySingleMigration(dbURL, name string, isUp bool) error {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	suffix := ".up.sql"
	if !isUp {
		suffix = ".down.sql"
	}
	glob := filepath.Join("database", "migrations", "*_"+name+suffix)
	matches, err := filepath.Glob(glob)
	if err != nil || len(matches) == 0 {
		return fmt.Errorf("no matching migration file found for '%s'", name)
	}

	content, err := os.ReadFile(matches[0])
	if err != nil {
		return err
	}

	stmts := strings.Split(string(content), ";")
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			_, err := db.Exec(stmt)
			if err != nil {
				return fmt.Errorf("error in statement: %s\n%w", stmt, err)
			}
		}
	}
	return nil
}

func createDatabase(user, pass, host, port, db string) error {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable", host, port, user, pass)
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer dbConn.Close()

	existsQuery := fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname='%s'", db)
	var exists int
	err = dbConn.QueryRow(existsQuery).Scan(&exists)
	if err == sql.ErrNoRows {
		_, err = dbConn.Exec(fmt.Sprintf("CREATE DATABASE \"%s\"", db))
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}
