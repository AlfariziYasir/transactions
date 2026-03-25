package repository

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type productRepo struct {
	db *pgxpool.Pool
}

func NewProductRepo(db *pgxpool.Pool) ports.ProductRepo {
	return &productRepo{
		db: db,
	}
}

func (r *productRepo) getExecutor(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *productRepo) Upsert(ctx context.Context, req *model.ProductReplicas) error {
	query := `
		insert into product_replicas (id, name, price, is_active, last_updated, version)
		values ($1,$2,$3,$4,$5,$6)
		on conflict (id) do update
		set
			name = excluded.name,
			price = excluded.price,
			is_active = excluded.is_active,
			last_updated = excluded.last_updated,
			version = product_replicas.version + 1
		where
			excluded.last_updated > product_replicas.last_updated
	`

	tag, err := r.getExecutor(ctx).Exec(ctx, query,
		req.ID,
		req.Name,
		req.Price,
		req.IsActive,
		req.LastUpdated,
		1,
	)
	if err != nil {
		return errorx.DbError(err, "failed to upsert product")
	}

	if tag.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeConflict, "event is stale/outdated, update ignored", nil)
	}

	return nil
}

func (r *productRepo) Get(ctx context.Context, ids []string) ([]*model.ProductReplicas, error) {
	query := psql.Select((&model.ProductReplicas{}).ToColumns()...).From((&model.ProductReplicas{}).Tablename()).
		Where(squirrel.Eq{"id": ids})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to build data query", err)
	}

	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.ProductReplicas])
	if err != nil {
		return nil, errorx.DbError(err, "failed to collect rows")
	}

	if len(results) == 0 {
		results = []*model.ProductReplicas{}
	}

	return results, nil
}
