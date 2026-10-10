package compatibility

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	"github.com/pressly/goose/v3"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Prototype only: D3 will implement the production adapter and input limits.
type policyRow struct {
	ID                     uint64
	Ptype                  string
	V0, V1, V2, V3, V4, V5 string
}

func (policyRow) TableName() string { return "casbin_rules" }

type policyAdapter struct{ db *gorm.DB }

var _ persist.Adapter = (*policyAdapter)(nil)

func newPolicyAdapter(db *gorm.DB) (*policyAdapter, error) {
	var row policyRow
	if err := db.Limit(1).Find(&row).Error; err != nil {
		return nil, err
	}
	return &policyAdapter{db: db}, nil
}

func makeRow(ptype string, rule []string) policyRow {
	values := [6]string{}
	copy(values[:], rule)
	return policyRow{Ptype: ptype, V0: values[0], V1: values[1], V2: values[2], V3: values[3], V4: values[4], V5: values[5]}
}

func (a *policyAdapter) LoadPolicy(m model.Model) error {
	var rows []policyRow
	if err := a.db.Order("id").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		values := []string{row.Ptype, row.V0, row.V1, row.V2, row.V3, row.V4, row.V5}
		for len(values) > 1 && values[len(values)-1] == "" {
			values = values[:len(values)-1]
		}
		if err := persist.LoadPolicyArray(values, m); err != nil {
			return err
		}
	}
	return nil
}

func (a *policyAdapter) SavePolicy(m model.Model) error {
	return a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&policyRow{}).Error; err != nil {
			return err
		}
		for _, section := range []string{"p", "g"} {
			for ptype, assertion := range m[section] {
				for _, rule := range assertion.Policy {
					row := makeRow(ptype, rule)
					if err := tx.Create(&row).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (a *policyAdapter) AddPolicy(_ string, ptype string, rule []string) error {
	row := makeRow(ptype, rule)
	return a.db.Create(&row).Error
}

func (a *policyAdapter) RemovePolicy(_ string, ptype string, rule []string) error {
	row := makeRow(ptype, rule)
	return a.db.Where("ptype = ? AND v0 = ? AND v1 = ? AND v2 = ? AND v3 = ? AND v4 = ? AND v5 = ?", row.Ptype, row.V0, row.V1, row.V2, row.V3, row.V4, row.V5).Delete(&policyRow{}).Error
}

func (a *policyAdapter) RemoveFilteredPolicy(_ string, ptype string, index int, values ...string) error {
	if index < 0 || index+len(values) > 6 {
		return errors.New("invalid policy filter")
	}
	query := a.db.Where("ptype = ?", ptype)
	for offset, value := range values {
		if value != "" {
			query = query.Where(fmt.Sprintf("v%d = ?", index+offset), value)
		}
	}
	return query.Delete(&policyRow{}).Error
}

const policyModel = `[request_definition]
r = sub, obj, act
[policy_definition]
p = sub, obj, act
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`

func TestPolicyDatabases(t *testing.T) {
	for _, backend := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(backend, func(t *testing.T) {
			var db *gorm.DB
			var err error
			dialect := goose.DialectSQLite3
			identity := "INTEGER PRIMARY KEY AUTOINCREMENT"
			options := &gorm.Config{TranslateError: true, Logger: logger.Default.LogMode(logger.Silent)}
			switch backend {
			case "sqlite":
				db = openSQLite(t, filepath.Join(t.TempDir(), "policy.db"))
			case "mysql", "postgres":
				dsn := os.Getenv("GOSEED_TEST_" + map[string]string{"mysql": "MYSQL", "postgres": "POSTGRES"}[backend] + "_DSN")
				if dsn == "" {
					if os.Getenv("GOSEED_REQUIRE_EXTERNAL") == "1" {
						t.Fatal("external database DSN is required")
					}
					t.Skip("run make compatibility-db for isolated real database checks")
				}
				if backend == "mysql" {
					db, err = gorm.Open(mysql.Open(dsn), options)
					dialect = goose.DialectMySQL
					identity = "BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT"
				} else {
					db, err = gorm.Open(postgres.Open(dsn), options)
					dialect = goose.DialectPostgres
					identity = "BIGSERIAL PRIMARY KEY"
				}
				if err != nil {
					t.Fatal(err)
				}
				pool, err := db.DB()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = pool.Close() })
			}
			if _, err := newPolicyAdapter(db); err == nil {
				t.Fatal("adapter silently accepted absent schema")
			}
			if db.Migrator().HasTable(&policyRow{}) {
				t.Fatal("adapter created a table")
			}
			pool, _ := db.DB()
			schema := fmt.Sprintf("CREATE TABLE casbin_rules (id %s, ptype VARCHAR(32) NOT NULL, v0 VARCHAR(100) NOT NULL, v1 VARCHAR(100) NOT NULL, v2 VARCHAR(100) NOT NULL, v3 VARCHAR(100) NOT NULL, v4 VARCHAR(100) NOT NULL, v5 VARCHAR(100) NOT NULL, UNIQUE (ptype,v0,v1,v2,v3,v4,v5));", identity)
			migrate(t, pool, dialect, schema)
			adapter, err := newPolicyAdapter(db)
			if err != nil {
				t.Fatal(err)
			}
			newEnforcer := func() *casbin.Enforcer {
				t.Helper()
				m, err := model.NewModelFromString(policyModel)
				if err != nil {
					t.Fatal(err)
				}
				e, err := casbin.NewEnforcer(m, adapter)
				if err != nil {
					t.Fatal(err)
				}
				return e
			}
			e := newEnforcer()
			if ok, err := e.Enforce("alice", "items", "read"); err != nil || ok {
				t.Fatalf("default deny: %v %v", ok, err)
			}
			if _, err := e.AddPolicy("reader", "items", "read"); err != nil {
				t.Fatal(err)
			}
			if _, err := e.AddGroupingPolicy("alice", "reader"); err != nil {
				t.Fatal(err)
			}
			if err := e.SavePolicy(); err != nil {
				t.Fatal(err)
			}
			reloaded := newEnforcer()
			if ok, err := reloaded.Enforce("alice", "items", "read"); err != nil || !ok {
				t.Fatalf("persisted allow: %v %v", ok, err)
			}
			if ok, err := reloaded.Enforce("alice", "items", "write"); err != nil || ok {
				t.Fatalf("action deny: %v %v", ok, err)
			}
			// Failure after DELETE must preserve the previously committed policy.
			broken, err := model.NewModelFromString(policyModel)
			if err != nil {
				t.Fatal(err)
			}
			broken["p"]["p"].Policy = [][]string{{"duplicate", "items", "read"}, {"duplicate", "items", "read"}}
			if err := adapter.SavePolicy(broken); !errors.Is(err, gorm.ErrDuplicatedKey) {
				t.Fatalf("expected duplicate failure: %v", err)
			}
			if ok, err := newEnforcer().Enforce("alice", "items", "read"); err != nil || !ok {
				t.Fatalf("atomic save rollback: %v %v", ok, err)
			}
			if _, err := reloaded.RemovePolicy("reader", "items", "read"); err != nil {
				t.Fatal(err)
			}
			if ok, err := newEnforcer().Enforce("alice", "items", "read"); err != nil || ok {
				t.Fatalf("persisted removal: %v %v", ok, err)
			}
		})
	}
}
