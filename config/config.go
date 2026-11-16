package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

type SystemConfig struct {
	Env      string `env:"ENV" envDefault:"development"`
	Port     string `env:"PORT" envDefault:"9999"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"INFO"`

	//DB
	DBHost      string `env:"DB_HOST" envDefault:"localhost"`
	DBUsername  string `env:"DB_USERNAME" envDefault:"postgres"`
	DBPassword  string `env:"DB_PASSWORD" envDefault:"postgres"`
	DBName      string `env:"DB_NAME" envDefault:"storybook"`
	DBPort      string `env:"DB_PORT" envDefault:"5432"`
	DBSSLMode   string `env:"DB_SSL_MODE" envDefault:"disable"`
	DBTimezone  string `env:"DB_TIMEZONE" envDefault:"UTC"`
	DBDebugMode bool   `env:"DB_DEBUG_MODE" envDefault:"false"`

	//Auth — JWT_SECRET has no default on purpose; the server refuses to start without it.
	JWTSecret       string        `env:"JWT_SECRET"`
	AccessTokenTTL  time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"30m"`
	RefreshTokenTTL time.Duration `env:"REFRESH_TOKEN_TTL" envDefault:"168h"`

	//Subscription auto-renew job

	//CORS — comma separated origins allowed to call the API from a browser. Empty (the default) means
	//same-origin only: the back-office frontend is served on the same origin as /api (STORY-5).
	CORSAllowOrigins []string `env:"CORS_ALLOW_ORIGINS" envSeparator:","`

	//Seed accounts (go run main.go seed) — mocked login per PRD, values only from env
	SeedParentName     string `env:"SEED_PARENT_NAME" envDefault:"Demo Parent"`
	SeedParentEmail    string `env:"SEED_PARENT_EMAIL"`
	SeedParentPassword string `env:"SEED_PARENT_PASSWORD"`
	SeedAdminName      string `env:"SEED_ADMIN_NAME" envDefault:"Demo Admin"`
	SeedAdminEmail     string `env:"SEED_ADMIN_EMAIL"`
	SeedAdminPassword  string `env:"SEED_ADMIN_PASSWORD"`
	SeedPlanPrice      int    `env:"SEED_PLAN_PRICE" envDefault:"49000"`
	SeedPlanBenefits   string `env:"SEED_PLAN_BENEFITS" envDefault:"Read all published books."`
}

func LoadEnv() *SystemConfig {
	cfg := SystemConfig{}
	err := godotenv.Load()
	if err != nil {
		fmt.Println(err, ", getting config from ENV")
	}
	err = env.Parse(&cfg)
	if err != nil {
		fmt.Println(err, ", getting config from ENV")
	}
	return &cfg
}
