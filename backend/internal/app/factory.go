package app

import (
	"errors"
	"fmt"
	"strings"

	authn "github.com/brandsrx/supay/internal/auth"
	appconfig "github.com/brandsrx/supay/internal/config"
	deliveryHTTP "github.com/brandsrx/supay/internal/delivery/http"
	authModule "github.com/brandsrx/supay/internal/delivery/http/modules/auth"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/usecase"
	"gorm.io/gorm"
)

// Option supplies already-owned dependencies, primarily for embedding and tests.
type Option func(*appOptions)
type appOptions struct {
	db    *gorm.DB
	hasDB bool
}

func WithDatabase(db *gorm.DB) Option { return func(o *appOptions) { o.db = db; o.hasDB = true } }

func NewApp(cfg appconfig.Config, options ...Option) (*App, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DeploymentMode)) {
	case "cloud":
		return NewCloudApp(cfg, options...)
	case "", "selfhosted":
		return NewSelfHostedApp(cfg, options...)
	default:
		return nil, fmt.Errorf("DEPLOYMENT_MODE inválido: %q", cfg.DeploymentMode)
	}
}

func NewCloudApp(cfg appconfig.Config, options ...Option) (*App, error) {
	cfg.DeploymentMode = "cloud"
	return newApp(cfg, cloudAuth, options...)
}

func NewSelfHostedApp(cfg appconfig.Config, options ...Option) (*App, error) {
	cfg.DeploymentMode = "selfhosted"
	return newApp(cfg, selfHostedAuth, options...)
}

type authFactory func(*App) deliveryHTTP.AuthOptions

func cloudAuth(c *App) deliveryHTTP.AuthOptions {
	cfg := c.cfg.BetterAuth
	return deliveryHTTP.AuthOptions{
		Tokens:      authn.NewBetterAuthVerifier(cfg.JWKSURL, cfg.Issuer, cfg.Audience, cfg.JWKSCacheTTL, cfg.HTTPTimeout),
		Memberships: postgres.NewBetterAuthMembershipRepository(c.db),
	}
}

func selfHostedAuth(c *App) deliveryHTTP.AuthOptions {
	jwt := authn.NewJWTManager(c.cfg.JWTSecret, c.cfg.JWTIssuer, c.cfg.JWTAccessTTL)
	c.authUsecase = usecase.NewAuthUsecase(c.authRepo, c.companyUsecase, jwt, authn.BcryptHasher{})
	return deliveryHTTP.AuthOptions{Tokens: jwt, Memberships: c.authRepo, Routes: authModule.NewModule(c.authUsecase)}
}

func newApp(cfg appconfig.Config, auth authFactory, options ...Option) (_ *App, err error) {
	if err = validateConfig(cfg); err != nil {
		return nil, err
	}
	var opts appOptions
	for _, option := range options {
		option(&opts)
	}
	c := &App{cfg: cfg, db: opts.db}
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
		}
	}()
	if !opts.hasDB {
		c.db, err = database.OpenForMode(cfg.DatabaseURL, cfg.DeploymentMode)
		if err != nil {
			return nil, fmt.Errorf("PostgreSQL: %w", err)
		}
		sqlDB, dbErr := c.db.DB()
		if dbErr != nil {
			return nil, dbErr
		}
		c.closers = append(c.closers, sqlDB.Close)
	}
	if c.db == nil {
		return nil, fmt.Errorf("PostgreSQL no configurado")
	}
	if cfg.AutoMigrate {
		if err = database.MigrateDB(c.db); err != nil {
			return nil, fmt.Errorf("migraciones: %w", err)
		}
	}
	configureRepositories(c)
	if err = configureStorage(c); err != nil {
		return nil, err
	}
	if cfg.RunMode.RunsEmailWorker() {
		if err = configureEmailWorker(c); err != nil {
			return nil, err
		}
	} else {
		if err = configureSIAT(c); err != nil {
			return nil, err
		}
		if err = configureEmailDispatcher(c); err != nil {
			return nil, err
		}
		if c.emailTasksClient != nil {
			c.closers = append(c.closers, c.emailTasksClient.Close)
		}
		configureServices(c)
		if err = configureQueue(c); err != nil {
			return nil, err
		}
		if cfg.RunMode.RunsAPI() {
			c.authOptions = auth(c)
		}
	}
	if cfg.RunMode.RunsWeb() {
		configureHTTP(c)
	}
	return c, nil
}

func validateConfig(cfg appconfig.Config) error {
	if err := cfg.ValidateRunMode(); err != nil {
		return err
	}
	if cfg.RunMode.RunsAPI() {
		if err := cfg.ValidateAuth(); err != nil {
			return err
		}
		if cfg.BackendSecret == "" {
			return fmt.Errorf("BACKEND_SECRET es obligatorio")
		}
	}
	if err := cfg.ValidateInvoiceEmail(); err != nil {
		return err
	}
	if err := cfg.ValidateStorage(); err != nil {
		return err
	}
	return cfg.ValidateAutoMigrate(cfg.AutoMigrate)
}
