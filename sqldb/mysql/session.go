package mysql

import (
	"database/sql"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/v2"
)

func NewSessionStore(db *sql.DB) scs.Store {

	db.Exec(`
		CREATE TABLE sessions (
			token CHAR(43) PRIMARY KEY,
			data BLOB NOT NULL,
			expiry TIMESTAMP(6) NOT NULL
		);

		CREATE INDEX sessions_expiry_idx ON sessions (expiry);`)

	return mysqlstore.New(db)
}
