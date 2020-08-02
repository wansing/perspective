package sqldb

import "database/sql"

func mustPrepare(db *sql.DB, query string) *sql.Stmt {
	stmt, err := db.Prepare(query)
	if err != nil {
		panic(err)
	}
	return stmt
}
