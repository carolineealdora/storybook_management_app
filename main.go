package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codebase/handler"
	"codebase/migrations"
	"codebase/models"
)

func main() {
	shared := models.SharedHolder{}
	shared.Init()

	args := os.Args
	if len(args) > 1 {
		switch args[1] {
		case "migrate":
			migrate(shared, args)
			return
		case "seed":
			if err := migrations.Seed(shared.Sql, shared.Config); err != nil {
				shared.Log.Fatal(err)
			}
			shared.Log.Info("seed complete")
			return
		case "run":
		default:
			fmt.Println("unknown command; use: run | seed | migrate up|down|create <name>")
			return
		}
	}
	run(&shared)
}

func migrate(shared models.SharedHolder, args []string) {
	if len(args) < 3 {
		shared.Log.Error("usage: migrate up | down [name] | create <name>")
		return
	}
	migrator := migrations.NewMigration(shared)
	var err error
	switch args[2] {
	case "up":
		err = migrator.Up()
	case "down":
		if len(args) <= 3 || len(args[3]) == 0 {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("[WARNING] Rollback All Migrations? (Y/N) ")
			char, _, rerr := reader.ReadRune()
			if rerr != nil || (char != 'y' && char != 'Y') {
				return
			}
			err = migrator.Down("")
		} else {
			err = migrator.Down(args[3])
		}
	case "create":
		if len(args) <= 3 || len(args[3]) == 0 {
			shared.Log.Error("please input new migration name")
			return
		}
		err = migrator.Create(args[3])
	}
	if err != nil {
		shared.Log.Error(err)
		os.Exit(1)
	}
}

func run(shared *models.SharedHolder) {
	if len(shared.Config.JWTSecret) < 32 {
		shared.Log.Fatal("JWT_SECRET must be set and at least 32 characters")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler.InitRoute(shared)

	go func() {
		if err := shared.Echo.Start("0.0.0.0:" + shared.Config.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shared.Log.Fatal(fmt.Errorf("failed to start echo: %w", err))
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := shared.Echo.Shutdown(shutdownCtx); err != nil {
		shared.Log.Error(err)
	}
	shared.Log.Info("server stopped")
}
