package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationFilesExist(t *testing.T) {
	root := filepath.Join("..", "..", "migrations")
	files := []string{"000001_init.up.sql", "000001_init.down.sql"}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("migration file missing %s: %v", f, err)
		}
	}
	// check sqlc generated
	if _, err := os.Stat(filepath.Join("db", "models.go")); err != nil {
		t.Fatalf("sqlc models.go missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join("db", "querier.go")); err != nil {
		t.Fatalf("sqlc querier.go missing: %v", err)
	}
}

func TestDockerComposeExists(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "docker-compose.yml")); err != nil {
		t.Fatalf("docker-compose.yml missing: %v", err)
	}
}

func TestSQLCConfigExists(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "sqlc.yaml")); err != nil {
		t.Fatalf("sqlc.yaml missing: %v", err)
	}
}
