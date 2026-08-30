package schema

import (
	"fmt"
	"strconv"

	"github.com/domainry/domainry-orm/dialect"
)

type ColumnType interface {
	renderColumnType(dialect.Name) (string, error)
}

type portableColumnType struct {
	name      string
	length    int
	precision int
	scale     int
	keyString bool
}

func Text() ColumnType      { return portableColumnType{name: "TEXT"} }
func LongText() ColumnType  { return portableColumnType{name: "LONGTEXT"} }
func BigInt() ColumnType    { return portableColumnType{name: "BIGINT"} }
func Integer() ColumnType   { return portableColumnType{name: "INTEGER"} }
func SmallInt() ColumnType  { return portableColumnType{name: "SMALLINT"} }
func Boolean() ColumnType   { return portableColumnType{name: "BOOLEAN"} }
func Real() ColumnType      { return portableColumnType{name: "REAL"} }
func Double() ColumnType    { return portableColumnType{name: "DOUBLE"} }
func Date() ColumnType      { return portableColumnType{name: "DATE"} }
func Time() ColumnType      { return portableColumnType{name: "TIME"} }
func Timestamp() ColumnType { return portableColumnType{name: "TIMESTAMP"} }
func JSON() ColumnType      { return portableColumnType{name: "JSON"} }
func UUID() ColumnType      { return portableColumnType{name: "UUID"} }
func Binary() ColumnType    { return portableColumnType{name: "BINARY"} }

func Decimal(precision, scale int) ColumnType {
	return portableColumnType{name: "DECIMAL", precision: precision, scale: scale}
}

func Varchar(length int) ColumnType {
	return portableColumnType{name: "VARCHAR", length: length}
}

// TextKey is an index-safe text identity. MySQL requires a bounded
// VARCHAR; PostgreSQL and SQLite use TEXT without leaking that distinction to
// schema owners.
func TextKey(maxLength int) ColumnType {
	return portableColumnType{name: "TEXT", length: maxLength, keyString: true}
}

func (t portableColumnType) renderColumnType(name dialect.Name) (string, error) {
	if t.keyString {
		if t.length < 1 || t.length > 65535 {
			return "", fmt.Errorf("SQL text key length must be between 1 and 65535")
		}
		if name == dialect.MySQL {
			return "VARCHAR(" + strconv.Itoa(t.length) + ")", nil
		}
		if name == dialect.Postgres || name == dialect.SQLite {
			return "TEXT", nil
		}
	}
	if t.name == "VARCHAR" {
		if t.length < 1 || t.length > 65535 {
			return "", fmt.Errorf("SQL varchar length must be between 1 and 65535")
		}
		return "VARCHAR(" + strconv.Itoa(t.length) + ")", nil
	}
	if t.name == "DECIMAL" {
		if t.precision < 1 || t.precision > 38 || t.scale < 0 || t.scale > t.precision {
			return "", fmt.Errorf("SQL decimal precision must be between 1 and 38 and scale between 0 and precision")
		}
		return "DECIMAL(" + strconv.Itoa(t.precision) + "," + strconv.Itoa(t.scale) + ")", nil
	}
	switch t.name {
	case "TEXT", "BIGINT", "INTEGER", "SMALLINT", "BOOLEAN", "REAL", "DATE", "TIME", "TIMESTAMP":
		return t.name, nil
	case "LONGTEXT":
		if name == dialect.MySQL {
			return "LONGTEXT", nil
		}
		return "TEXT", nil
	case "DOUBLE":
		if name == dialect.Postgres {
			return "DOUBLE PRECISION", nil
		}
		return "DOUBLE", nil
	case "JSON":
		switch name {
		case dialect.Postgres:
			return "JSONB", nil
		case dialect.MySQL:
			return "JSON", nil
		case dialect.SQLite:
			return "TEXT", nil
		}
	case "UUID":
		if name == dialect.Postgres {
			return "UUID", nil
		}
		return "VARCHAR(36)", nil
	case "BINARY":
		switch name {
		case dialect.Postgres:
			return "BYTEA", nil
		case dialect.MySQL:
			return "LONGBLOB", nil
		case dialect.SQLite:
			return "BLOB", nil
		}
	default:
		return "", fmt.Errorf("unsupported SQL column type %q", t.name)
	}
	return "", fmt.Errorf("unsupported SQL column type %q for dialect %q", t.name, name)
}
