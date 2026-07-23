package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tgdrive/teldrive/internal/database/jet/gen/model"
	"github.com/tgdrive/teldrive/internal/database/jet/gen/table"
)

type JetUserRepository struct {
	db jetDB
}

func NewJetUserRepository(pool *pgxpool.Pool) *JetUserRepository {
	return &JetUserRepository{db: newJetDB(pool)}
}

func (r *JetUserRepository) Create(ctx context.Context, user *model.Users) error {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}

	stmt := table.Users.
		INSERT(table.Users.AllColumns).
		MODEL(*user).
		ON_CONFLICT(table.Users.UserID).
		DO_NOTHING()

	err := r.db.exec(ctx, stmt)

	return err
}

func (r *JetUserRepository) GetByID(ctx context.Context, userID int64) (*model.Users, error) {

	stmt := table.Users.SELECT(table.Users.AllColumns).FROM(table.Users).WHERE(table.Users.UserID.EQ(postgres.Int64(userID)))

	var out model.Users
	if err := r.db.query(ctx, stmt, &out); err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &out, nil
}

func (r *JetUserRepository) GetByIDForUpdate(ctx context.Context, userID int64) (*model.Users, error) {
	var out model.Users
	err := r.db.executor(ctx).QueryRow(ctx, `
		SELECT user_id, name, user_name, is_premium, created_at, updated_at, encrypt_files
		FROM users
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(
		&out.UserID,
		&out.Name,
		&out.UserName,
		&out.IsPremium,
		&out.CreatedAt,
		&out.UpdatedAt,
		&out.EncryptFiles,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, normalizeDBError(err)
	}

	return &out, nil
}

func (r *JetUserRepository) Update(ctx context.Context, userID int64, update UserUpdate) error {
	updates := make([]postgres.ColumnAssigment, 0, 5)

	if update.Name != nil {
		updates = append(updates, table.Users.Name.SET(postgres.String(*update.Name)))
	}
	if update.UserName != nil {
		updates = append(updates, table.Users.UserName.SET(postgres.String(*update.UserName)))
	}
	if update.IsPremium != nil {
		updates = append(updates, table.Users.IsPremium.SET(postgres.Bool(*update.IsPremium)))
	}
	if update.EncryptFiles != nil {
		updates = append(updates, table.Users.EncryptFiles.SET(postgres.Bool(*update.EncryptFiles)))
	}

	updatedAt := time.Now().UTC()
	if update.UpdatedAt != nil {
		updatedAt = *update.UpdatedAt
	}
	updates = append(updates, table.Users.UpdatedAt.SET(postgres.TimestampzT(updatedAt)))

	stmt := table.Users.UPDATE().WHERE(table.Users.UserID.EQ(postgres.Int64(userID)))
	stmt = stmt.SET(updates[0], assignmentArgs(updates[1:])...)

	err := r.db.exec(ctx, stmt)

	return err
}

func (r *JetUserRepository) Exists(ctx context.Context, userID int64) (bool, error) {
	stmt := postgres.SELECT(postgres.COUNT(table.Users.UserID)).FROM(table.Users).WHERE(table.Users.UserID.EQ(postgres.Int64(userID)))

	var row struct {
		Count int64
	}
	if err := r.db.query(ctx, stmt, &row); err != nil {
		return false, err
	}

	return row.Count > 0, nil
}

func (r *JetUserRepository) All(ctx context.Context) ([]model.Users, error) {
	stmt := table.Users.SELECT(table.Users.AllColumns).FROM(table.Users).ORDER_BY(table.Users.UserID.ASC())

	var out []model.Users
	if err := r.db.query(ctx, stmt, &out); err != nil {
		return nil, err
	}

	return out, nil
}
