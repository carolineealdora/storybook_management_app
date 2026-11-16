package models

import (
	"codebase/config"
	"codebase/pkg/logger"
	"codebase/pkg/logger/logrus"
	sql "codebase/pkg/orm"
	"codebase/pkg/orm/postgres"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

const (
	CommonTimeStampFormat = "2006-01-02 15:04:05.000Z07:00"
	CommonDateFormat      = "2006-01-02"
	DDMMYYFormat          = "020106"
	LogPattern            = "[%s] %v"
)

type (
	SharedHolder struct {
		Config *config.SystemConfig
		Log    logger.Logger
		Echo   *echo.Echo
		Sql    sql.ORM
	}

	Model interface {
		TableName() string
		Validate() error
	}
)

func (h *SharedHolder) Init() {
	h.Config = config.LoadEnv()
	h.Log = initLog(*h.Config)
	h.Echo = initEcho(*h.Config)
	h.Sql = initSql(h.Log, *h.Config)
}

func initLog(cfg config.SystemConfig) logger.Logger {
	option := logrus.Option{
		Level:       logrus.Level(cfg.LogLevel),
		LogFilePath: "logs/",
		Formatter:   logrus.JSONFormatter,
		MaxSize:     1,
		MaxBackups:  3,
		MaxAge:      -1,
		Compress:    false,
	}
	logs, _ := logrus.New(&option)
	return logs
}

func initEcho(cfg config.SystemConfig) *echo.Echo {
	e := echo.New()
	e.Logger.SetLevel(log.DEBUG)
	if cfg.Env == "development" {
		e.Debug = true
	}
	return e
}

func initSql(log logger.Logger, sysConfig config.SystemConfig) sql.ORM {
	connection := sql.Connection{
		Host:     sysConfig.DBHost,
		Username: sysConfig.DBUsername,
		Password: sysConfig.DBPassword,
		DBName:   sysConfig.DBName,
		Port:     sysConfig.DBPort,
		SSLMode:  sysConfig.DBSSLMode,
		Timezone: sysConfig.DBTimezone,
		SQLDebug: sysConfig.DBDebugMode,
		ORMDebug: sysConfig.DBDebugMode,
	}

	sql, err := postgres.NewPostgresConnection(connection, log)
	if err != nil {
		log.Fatalf("failed to connect to database %s@%s:%s/%s: %v", connection.Username, connection.Host, connection.Port, connection.DBName, err)
		return nil
	}

	return sql
}
