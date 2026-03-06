package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/Masterminds/squirrel"
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
		ToSql()

	res, err := r.getExecutor(ctx).Exec(ctx, query, args)
	if err != nil {
		if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key value") {
			return false, nil
		}

		return false, errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return false, errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return true, nil
}

func (r *inboxRepo) Get(ctx context.Context, messageId string) (bool, error) {
	query, args, _ := psql.Select("1").
		From((&model.Inbox{}).TableName()).
		Where(squirrel.Eq{"message_id": messageId}).
		ToSql()

	var found int
	err := r.getExecutor(ctx).QueryRow(ctx, query, args...).Scan(&found)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}

		return false, errorx.DbError(err, err.Error())
	}

	return true, nil
}
