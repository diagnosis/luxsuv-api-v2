package app

import (
	"context"
	"os"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/api"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
	"github.com/diagnosis/luxsuv-api-v2/internal/secure"
	"github.com/diagnosis/luxsuv-api-v2/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Application struct {
	DB            *pgxpool.Pool
	Signer        *secure.Signer
	HealthHandler *api.HealthHandler
	UserHandler   *api.UserHandler
}

func NewApplication(pool *pgxpool.Pool) (*Application, error) {
	ctx := context.Background()
	logger.Info(ctx, "initializing application")

	healthHandler := api.NewHealthHandler()

	userStore := store.NewPostgresUserStore(pool)
	refreshTokenStore := store.NewPostgresRefreshTokenStore(pool)
	accessSecret := []byte(os.Getenv("JWT_ACCESS_SECRET"))
	refreshSecret := []byte(os.Getenv("JWT_REFRESH_SECRET"))
	issuer := os.Getenv("TOKEN_ISSUER")
	audience := "Lux suv apps"

	if len(accessSecret) < 32 || len(refreshSecret) < 32 {
		logger.Error(ctx, "JWT secrets are invalid", "access_len", len(accessSecret), "refresh_len", len(refreshSecret))
		return nil, secure.ErrSecretsInvalid
	}

	signer, err := secure.NewSigner(issuer, audience, accessSecret, refreshSecret, 15*time.Second, 7*24*time.Hour)
	if err != nil {
		logger.Error(ctx, "failed to create JWT signer", "error", err)
		return nil, err
	}
	logger.Info(ctx, "JWT signer initialized", "issuer", issuer, "audience", audience)

	userHandler := api.NewUserHandler(userStore, signer, refreshTokenStore)

	logger.Info(ctx, "application initialized successfully")

	return &Application{
		pool, signer, healthHandler, userHandler,
	}, nil

}
