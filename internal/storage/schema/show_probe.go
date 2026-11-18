package schema

import (
	"context"
	"database/sql"
	"strings"
)

// Existence probes built on SHOW rather than INFORMATION_SCHEMA.
//
// Dolt does not push the TABLE_SCHEMA = DATABASE() predicate into
// INFORMATION_SCHEMA, so every probe there walks the catalog of every database
// on the server. On a shared server with dozens of databases a single
// INFORMATION_SCHEMA.COLUMNS probe took minutes, and these probes run on every
// store open. SHOW FULL TABLES and SHOW COLUMNS read only the current database.
//
// SHOW FULL TABLES never errors on an absent name, which matters: a failing
// statement pins a Dolt session to its pre-statement catalog snapshot (see
// currentVersion). showColumnExists therefore checks the table first and only
// issues SHOW COLUMNS against a table known to exist.

// showTableExists is upstream's, in schema.go (SHOW TABLES, case-insensitive).

// showColumnExists reports whether table.column exists in the current
// database. An absent table reports false, as INFORMATION_SCHEMA did.
func showColumnExists(ctx context.Context, db DBConn, table, column string) (bool, error) {
	present, err := showTableExists(ctx, db, table)
	if err != nil || !present {
		return false, err
	}
	return showHasFirstCell(ctx, db, "SHOW COLUMNS FROM `"+strings.ReplaceAll(table, "`", "``")+"`", column)
}

// showHasFirstCell runs a SHOW statement and reports whether any row's first
// cell equals name, ignoring case.
func showHasFirstCell(ctx context.Context, db DBConn, query, name string) (bool, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	cells := make([]sql.RawBytes, len(cols))
	dest := make([]any, len(cols))
	for i := range cells {
		dest[i] = &cells[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return false, err
		}
		if len(cells) > 0 && strings.EqualFold(string(cells[0]), name) {
			return true, nil
		}
	}
	return false, rows.Err()
}
