package compatibility

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/libtnb/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openSQLite(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(150)&_pragma=journal_mode(WAL)"), &gorm.Config{TranslateError: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	pool.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = pool.Close() })
	return db
}

func migrate(t *testing.T, pool *sql.DB, dialect goose.Dialect, schema string) {
	t.Helper()
	files := fstest.MapFS{"00001_schema.sql": &fstest.MapFile{Data: []byte("-- +goose Up\n" + schema + "\n-- +goose Down\nSELECT 1;\n")}}
	provider, err := goose.NewProvider(dialect, pool, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	if results, err := provider.Up(context.Background()); err != nil || len(results) != 0 {
		t.Fatalf("migration replay: results=%d error=%v", len(results), err)
	}
}

func TestSQLiteBehavior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.db")
	db := openSQLite(t, path)
	pool, _ := db.DB()
	ctx := context.Background()
	// Hold distinct connections at once to verify per-connection initialization.
	var connections []*sql.Conn
	for range 3 {
		connection, err := pool.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, connection)
		var foreignKeys, busyTimeout int
		var journal string
		if err := connection.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatal(err)
		}
		if err := connection.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatal(err)
		}
		if err := connection.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
			t.Fatal(err)
		}
		if foreignKeys != 1 || busyTimeout != 150 || journal != "wal" {
			t.Fatalf("connection PRAGMA: %d %d %s", foreignKeys, busyTimeout, journal)
		}
	}
	for _, connection := range connections {
		if err := connection.Close(); err != nil {
			t.Fatal(err)
		}
	}
	migrate(t, pool, goose.DialectSQLite3, "CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE); CREATE TABLE children (item_id INTEGER NOT NULL REFERENCES items(id));")
	if err := db.Exec("INSERT INTO items (id, name) VALUES (1, 'persisted')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO items (id, name) VALUES (2, 'persisted')").Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("duplicate translation: %v", err)
	}
	if err := db.Exec("INSERT INTO children (item_id) VALUES (999)").Error; !errors.Is(err, gorm.ErrForeignKeyViolated) {
		t.Fatalf("foreign key translation: %v", err)
	}
	sentinel := errors.New("rollback")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO items (id, name) VALUES (3, 'rolled-back')").Error; err != nil {
			return err
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("items").Where("id = ?", 3).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rollback: count=%d error=%v", count, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := db.WithContext(canceled).Exec("INSERT INTO items VALUES (4, 'canceled')").Error; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
	deadline, stop := context.WithTimeout(ctx, 20*time.Millisecond)
	defer stop()
	var total int64
	err := db.WithContext(deadline).Raw("WITH RECURSIVE seq(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM seq WHERE x<100000000) SELECT sum(x) FROM seq").Scan(&total).Error
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("running query deadline: %v", err)
	}
	// Separate pool permits a second writer while the first owns a transaction.
	other := openSQLite(t, path)
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	if err := tx.Exec("INSERT INTO items VALUES (5, 'locked')").Error; err != nil {
		_ = tx.Rollback().Error
		t.Fatal(err)
	}
	started := time.Now()
	err = other.Exec("INSERT INTO items VALUES (6, 'blocked')").Error
	elapsed := time.Since(started)
	if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if err == nil || !strings.Contains(err.Error(), "SQLITE_BUSY") || elapsed < 100*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("busy timeout: duration=%s error=%v", elapsed, err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopen after closing every original connection to prove file persistence.
	reopened := openSQLite(t, path)
	var name string
	if err := reopened.Raw("SELECT name FROM items WHERE id=1").Scan(&name).Error; err != nil || name != "persisted" {
		t.Fatalf("file persistence: %s %v", name, err)
	}
}
