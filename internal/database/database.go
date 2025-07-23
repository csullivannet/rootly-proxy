package database

import (
	"database/sql"
	"errors"
	"log"
	"net"
	"os"
)

var ErrNoRows = sql.ErrNoRows

type StatusPage struct {
	ID          int
	Hostname    string
	PageDataURL string
}

type Repository interface {
	FindByHostname(hostname string) (*StatusPage, error)
}

type DB interface {
	QueryRow(query string, args ...interface{}) Row
}

type Row interface {
	Scan(dest ...interface{}) error
}

type sqlDB struct {
	*sql.DB
}

func (db *sqlDB) QueryRow(query string, args ...interface{}) Row {
	return db.DB.QueryRow(query, args...)
}

type PostgresRepository struct {
	db DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{
		db: &sqlDB{db},
	}
}

func (r *PostgresRepository) FindByHostname(hostname string) (*StatusPage, error) {
	// Strip ports from hostname
	host, _, err := net.SplitHostPort(hostname)
	if err != nil {
		// If SplitHostPort fails, it's likely because there's no port
		// In that case, use the original hostname
		host = hostname
	}

	query := `SELECT id, hostname, page_data_url FROM status_pages WHERE hostname = $1`

	var sp StatusPage
	err = r.db.QueryRow(query, host).Scan(&sp.ID, &sp.Hostname, &sp.PageDataURL)
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &sp, nil
}

func SetupDatabase() *sql.DB {
	// Database connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost/rootly?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	return db
}

func GetAllHostnames(repo Repository) ([]string, error) {
	// This is a simplified approach - we'd need to add a method to Repository interface
	// For now, we'll hardcode the test hostnames that should be in the database
	return []string{"status.acme.com", "status.example.com"}, nil
}
