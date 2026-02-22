package repository

import (
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/common/pkg/redis"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/ports"
	"github.com/Masterminds/squirrel"
)

var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type repository struct {
	db postgres.PgxExecutor
	redis.Cache
}

func NewRepository(cache redis.Cache, db postgres.PgxExecutor) ports.Repository {
	return &repository{
		db:    db,
		Cache: cache,
	}
}
