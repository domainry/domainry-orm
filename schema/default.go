package schema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/domainry/domainry-orm/dialect"
)

func renderColumnDefault(name dialect.Name, value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "current_timestamp":
		return "CURRENT_TIMESTAMP", nil
	case "false":
		if name == dialect.SQLite {
			return "0", nil
		}
		return "FALSE", nil
	case "true":
		if name == dialect.SQLite {
			return "1", nil
		}
		return "TRUE", nil
	case "empty_json":
		switch name {
		case dialect.Postgres:
			return "'{}'::jsonb", nil
		case dialect.MySQL:
			return "(JSON_OBJECT())", nil
		case dialect.SQLite:
			return "'{}'", nil
		}
	}
	return "", fmt.Errorf("unsupported SQL column default %q", value)
}

func renderColumnDefaultLiteral(name dialect.Name, value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return "'" + strings.ReplaceAll(typed, "'", "''") + "'", nil
	case bool:
		if name == dialect.SQLite {
			if typed {
				return "1", nil
			}
			return "0", nil
		}
		if typed {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	default:
		return "", fmt.Errorf("unsupported SQL column default literal %T", value)
	}
}
