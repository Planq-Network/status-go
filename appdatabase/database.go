package appdatabase

import (
	"database/sql"

	"github.com/planq-network/status-go/appdatabase/migrations"
	migrationsprevnodecfg "github.com/planq-network/status-go/appdatabase/migrationsprevnodecfg"
	"github.com/planq-network/status-go/nodecfg"
	"github.com/planq-network/status-go/sqlite"
)

const nodeCfgMigrationDate = 1640111208

// InitializeDB creates db file at a given path and applies migrations.
func InitializeDB(path, password string) (*sql.DB, error) {
	db, err := sqlite.OpenDB(path, password)
	if err != nil {
		return nil, err
	}

	// Check if the migration table exists
	row := db.QueryRow("SELECT exists(SELECT name FROM sqlite_master WHERE type='table' AND name='status_go_schema_migrations')")
	migrationTableExists := false
	err = row.Scan(&migrationTableExists)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var lastMigration uint64 = 0
	if migrationTableExists {
		row = db.QueryRow("SELECT version FROM status_go_schema_migrations")
		err = row.Scan(&lastMigration)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}

	if !migrationTableExists || (lastMigration > 0 && lastMigration < nodeCfgMigrationDate) {
		// If it's the first time migration's being run, or latest migration happened before migrating the nodecfg table
		err = migrationsprevnodecfg.Migrate(db)
		if err != nil {
			return nil, err
		}

		// NodeConfig migration cannot be done with SQL
		err = nodecfg.MigrateNodeConfig(db)
		if err != nil {
			return nil, err
		}
	}

	err = migrations.Migrate(db)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// DecryptDatabase creates an unencrypted copy of the database and copies it
// over to the given directory
func DecryptDatabase(oldPath, newPath, password string) error {
	return sqlite.DecryptDB(oldPath, newPath, password)
}

// EncryptDatabase creates an encrypted copy of the database and copies it to the
// user path
func EncryptDatabase(oldPath, newPath, password string) error {
	return sqlite.EncryptDB(oldPath, newPath, password)
}

func ChangeDatabasePassword(path, password, newPassword string) error {
	return sqlite.ChangeEncryptionKey(path, password, newPassword)
}
