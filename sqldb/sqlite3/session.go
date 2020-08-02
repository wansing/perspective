package sqlite3

import (
	"database/sql"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

func NewSessionStore(db *sql.DB) scs.Store {

	db.Exec(`
		CREATE TABLE sessions (
			token TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			expiry REAL NOT NULL
		);

		CREATE INDEX sessions_expiry_idx ON sessions(expiry);`)

	return sqlite3store.New(db)
}
