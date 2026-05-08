package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/controlplane"
	"github.com/jpaljasma/ecoflow-pulse/internal/logredact"
	"github.com/jpaljasma/ecoflow-pulse/internal/pgsearchpath"
	pulselog "github.com/jpaljasma/ecoflow-pulse/pkg/logger"
)

type config struct {
	dsn         string
	email       string
	userSubject string
}

func main() {
	logCfg := pulselog.DefaultServiceConfig("user-subject-reconcile")
	logCfg.Level = pulselog.ParseLevel(os.Getenv("LOG_LEVEL"), slog.LevelInfo)
	log, asyncLogHandler, err := pulselog.BuildServiceLogger(logCfg)
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger failed: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() {
		if asyncLogHandler != nil {
			asyncLogHandler.Close()
		}
	}()

	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		log.Error("invalid configuration", "error", err.Error())
		os.Exit(1)
	}

	dsn, err := pgsearchpath.ApplyFromEnv(cfg.dsn, "")
	if err != nil {
		log.Error("apply postgres search_path failed", "error", err.Error())
		os.Exit(1)
	}

	store, err := controlplane.NewPostgresStore(dsn)
	if err != nil {
		log.Error("open control-plane store failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	user, err := store.ReconcileUserSubjectByEmail(ctx, controlplane.ReconcileUserSubjectByEmailInput{
		Email:       cfg.email,
		UserSubject: cfg.userSubject,
	})
	if err != nil {
		switch {
		case errors.Is(err, controlplane.ErrVerifiedEmailNotFound):
			log.Error("verified email was not found", "email_ref", logredact.Email(cfg.email))
		case errors.Is(err, controlplane.ErrUserSubjectConflict):
			log.Error("target subject already belongs to another user", "user_subject", cfg.userSubject)
		default:
			log.Error("user subject reconciliation failed", "error", err.Error())
		}
		os.Exit(1)
	}

	log.Info("user subject reconciliation completed",
		"email", user.Email,
		"user_subject", user.KeycloakSubject,
		"display_name", user.DisplayName,
	)
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("ecoflow-user-subject-reconcile", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := config{}
	fs.StringVar(&cfg.dsn, "dsn", strings.TrimSpace(os.Getenv("CONTROL_PLANE_DB_DSN")), "Postgres DSN for the control-plane database")
	fs.StringVar(&cfg.email, "email", "", "Verified user email to match")
	fs.StringVar(&cfg.userSubject, "user-subject", "", "New Keycloak subject to assign")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.dsn) == "" {
		return config{}, errors.New("CONTROL_PLANE_DB_DSN or -dsn is required")
	}
	cfg.email = strings.TrimSpace(cfg.email)
	if cfg.email == "" {
		return config{}, errors.New("-email is required")
	}
	cfg.userSubject = strings.TrimSpace(cfg.userSubject)
	if cfg.userSubject == "" {
		return config{}, errors.New("-user-subject is required")
	}
	return cfg, nil
}
