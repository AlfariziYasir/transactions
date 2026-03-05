//go:build integration
// +build integration

package repository

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/AlfariziYasir/transactions/services/user/internal/core/model"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
)

var dbPool *pgxpool.Pool

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not construct pool: %s", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15-alpine",
		Env:        []string{"POSTGRES_PASSWORD=secret", "POSTGRES_USER=user", "POSTGRES_DB=test_user_db"},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	hostAndPort := resource.GetHostPort("5432/tcp")
	databaseUrl := fmt.Sprintf("postgres://user:secret@%s/test_user_db?sslmode=disable", hostAndPort)

	if err := pool.Retry(func() error {
		dbPool, err = pgxpool.New(context.Background(), databaseUrl)
		if err != nil {
			return err
		}
		return dbPool.Ping(context.Background())
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	_, err = dbPool.Exec(context.Background(), `
		CREATE TABLE users (
			id UUID PRIMARY KEY,
			name VARCHAR(50) NOT NULL,
			email VARCHAR(100) NOT NULL UNIQUE,
			password TEXT NOT NULL,
			role VARCHAR(10) NOT NULL DEFAULT 'user',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP,
			deleted_at TIMESTAMP
		);
	`)
	if err != nil {
		log.Fatalf("Could not setup tables: %s", err)
	}

	code := m.Run()

	// 7. Cleanup
	dbPool.Close()
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func TestUserRepository_CRUD(t *testing.T) {
	repo := NewRepository(dbPool)
	ctx := context.Background()

	user := &model.User{
		ID:       "a1111111-1111-1111-1111-111111111111",
		Name:     "Integration Test",
		Email:    "test@integration.com",
		Password: "hashed_password_example",
		Role:     "user",
	}

	t.Run("CreateUser", func(t *testing.T) {
		err := repo.Create(ctx, user)
		assert.NoError(t, err)
	})

	t.Run("GetByEmail", func(t *testing.T) {
		var existsUser model.User

		err := repo.Get(ctx, map[string]any{}, true, &existsUser)
		assert.NoError(t, err)
		assert.Equal(t, user.Name, existsUser.Name)
	})

	t.Run("GetByEmail_NotFound", func(t *testing.T) {
		res, count, err := repo.List(ctx, uint64(10), uint64(0), map[string]any{})
		assert.Error(t, err)
		assert.NotZero(t, count)
		assert.Nil(t, res)
	})
}
