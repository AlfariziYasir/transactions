package repository

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/jackc/pgx/v5"
)

type inboxRepo struct {
	db postgres.PgxExecutor
}

func NewInboxRepo(db postgres.PgxExecutor) ports.InboxRepo {
	return &inboxRepo{
		db: db,
	}
}

func (r *inboxRepo) getExecutor(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *inboxRepo) Create(ctx context.Context, inbox *model.Inbox) (bool, error) {
	query, args, _ := psql.Insert(inbox.TableName()).
		Columns(inbox.Columns()...).
		Values(inbox.ToRow()...).
		Suffix("on conflict (message_id) do nothing").
		ToSql()

	res, err := r.getExecutor(ctx).Exec(ctx, query, args...)
	if err != nil {
		return false, errorx.DbError(err, "failed to execute insert inbox")
	}

	if res.RowsAffected() == 0 {
		return false, nil
	}

	return true, nil
}
