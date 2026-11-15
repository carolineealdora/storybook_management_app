package postgres

import (
	"context"
	"fmt"
	"strings"

	"codebase/pkg/logger"
	"codebase/pkg/monitor"
	sql "codebase/pkg/orm"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

type gormDB struct {
	db *gorm.DB
}

func NewPostgresConnection(conn sql.Connection, log logger.Logger) (sql.ORM, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		conn.Host, conn.Username, conn.Password, conn.DBName, conn.Port, conn.SSLMode, conn.Timezone,
	)

	return open(dsn, conn.ORMDebug)
}

// NewPostgresDSN connects with a raw DSN and a silent ORM logger (used by integration tests).
func NewPostgresDSN(dsn string) (sql.ORM, error) {
	return open(dsn, false)
}

func open(dsn string, debug bool) (sql.ORM, error) {
	cfg := &gorm.Config{}
	if !debug {
		cfg.Logger = gormlogger.Default.LogMode(gormlogger.Silent)
	}
	db, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		return nil, err
	}
	return &gormDB{db: db}, nil
}

func (g *gormDB) Where(condition interface{}, args ...interface{}) sql.ORM {
	return &gormDB{db: g.db.Where(condition, args...)}
}

func (g *gormDB) OrWhere(query interface{}, args ...interface{}) sql.ORM {
	return &gormDB{db: g.db.Or(query, args...)}
}

func (g *gormDB) First(result interface{}) error {
	return g.db.First(result).Error
}

func (g *gormDB) All(result interface{}) (int64, error) {
	tx := g.db.Find(result)
	return tx.RowsAffected, tx.Error
}

func (g *gormDB) Order(order interface{}) sql.ORM {
	return &gormDB{db: g.db.Order(order)}
}

func (g *gormDB) Limit(limit int) sql.ORM {
	return &gormDB{db: g.db.Limit(limit)}
}

func (g *gormDB) Offset(offset int) sql.ORM {
	return &gormDB{db: g.db.Offset(offset)}
}

func (g *gormDB) Create(data interface{}) error {
	return g.db.Create(data).Error
}

func (g *gormDB) Update(data interface{}) error {
	return g.db.Save(data).Error
}

func (g *gormDB) Patch(data interface{}, fields ...string) error {
	if len(fields) > 0 {
		return g.db.Select(fields).Updates(data).Error
	}
	return g.db.Updates(data).Error
}

func (g *gormDB) Delete(data interface{}, args ...interface{}) error {
	return g.db.Delete(data, args...).Error
}

func (g *gormDB) Upsert(chunkSize int, data interface{}, onConflict sql.OnConflict) error {
	cc := clause.OnConflict{
		DoNothing: onConflict.DoNothing,
		UpdateAll: onConflict.UpdateAll,
	}
	for _, col := range onConflict.Columns {
		cc.Columns = append(cc.Columns, clause.Column{Name: col})
	}
	return g.db.Clauses(cc).CreateInBatches(data, chunkSize).Error
}

func (g *gormDB) Exec(query string, args ...interface{}) error {
	return g.db.Exec(query, args...).Error
}

func (g *gormDB) RawSql(query string, result interface{}, args ...interface{}) error {
	return g.db.Raw(query, args...).Scan(result).Error
}

func (g *gormDB) FindByQuery(tableName string, selectFields []string, where []sql.Query, result interface{}) sql.ORM {
	db := g.db.Table(tableName)
	if len(selectFields) > 0 {
		db = db.Select(selectFields)
	}
	for _, q := range where {
		db = db.Where(fmt.Sprintf("%s %s ?", q.Field, q.Operator), q.Value)
	}
	db = db.Find(result)
	return &gormDB{db: db}
}

func (g *gormDB) FillQuery(tableName string, query []sql.Query) sql.ORM {
	db := g.db.Table(tableName)
	for _, q := range query {
		db = db.Where(fmt.Sprintf("%s %s ?", q.Field, q.Operator), q.Value)
	}
	return &gormDB{db: db}
}

func (g *gormDB) Count() int64 {
	var count int64
	g.db.Count(&count)
	return count
}

func (g *gormDB) Select(fields []string) sql.ORM {
	return &gormDB{db: g.db.Select(fields)}
}

func (g *gormDB) Sort(sorts sql.Sorts) sql.ORM {
	db := g.db
	for _, s := range sorts {
		db = db.Order(s.Field + " " + s.Order)
	}
	return &gormDB{db: db}
}

func (g *gormDB) Join(joins sql.Joins) sql.ORM {
	joinType := joins.Type
	if joinType == "" {
		joinType = "INNER"
	}
	joinClause := fmt.Sprintf("%s JOIN %s ON %s", joinType, joins.Table, joins.Condition)
	return &gormDB{db: g.db.Joins(joinClause)}
}

func (g *gormDB) Group(fields []string) sql.ORM {
	return &gormDB{db: g.db.Group(strings.Join(fields, ", "))}
}

func (g *gormDB) HasTable(tableName string) bool {
	return g.db.Migrator().HasTable(tableName)
}

func (g *gormDB) CreateTable(data interface{}) error {
	return g.db.Migrator().CreateTable(data)
}

func (g *gormDB) CreateTableWithName(name string, data interface{}) error {
	return g.db.Table(name).Migrator().CreateTable(data)
}

func (g *gormDB) DropTable(data interface{}) error {
	return g.db.Migrator().DropTable(data)
}

func (g *gormDB) DropTableWithName(name string, data interface{}) error {
	return g.db.Migrator().DropTable(name)
}

func (g *gormDB) HasColumn(data interface{}, column string) bool {
	return g.db.Migrator().HasColumn(data, column)
}

func (g *gormDB) AddColumn(data interface{}, column string) error {
	return g.db.Migrator().AddColumn(data, column)
}

func (g *gormDB) DropColumn(data interface{}, column string) error {
	return g.db.Migrator().DropColumn(data, column)
}

func (g *gormDB) AlterColumn(data interface{}, column string) error {
	return g.db.Migrator().AlterColumn(data, column)
}

func (g *gormDB) Table(tableName string) sql.ORM {
	return &gormDB{db: g.db.Table(tableName)}
}

func (g *gormDB) Begin() sql.ORM {
	return &gormDB{db: g.db.Begin()}
}

func (g *gormDB) Commit() error {
	return g.db.Commit().Error
}

func (g *gormDB) Rollback() error {
	return g.db.Rollback().Error
}

func (g *gormDB) Ping() error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (g *gormDB) Close() error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (g *gormDB) Error() error {
	return g.db.Error
}

func (g *gormDB) RowsAffected() int64 {
	return g.db.RowsAffected
}

func (g *gormDB) Monitor(_ context.Context, _ monitor.Monitor, _ string, _ bool) sql.ORM {
	return g
}
