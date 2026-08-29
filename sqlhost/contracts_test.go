package sqlhost

import "database/sql"

func acceptsDatabase(Database) {}
func acceptsDBTX(DBTX)         {}

func compileContracts(db *sql.DB, tx *sql.Tx) {
	acceptsDatabase(db)
	acceptsDBTX(db)
	acceptsDBTX(tx)
}
