package model

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type User struct {
	ID        string             `db:"id"`
	Name      string             `db:"name"`
	Email     string             `db:"email"`
	Password  string             `db:"password"`
	Role      string             `db:"role"`
	CreatedAt pgtype.Timestamptz `db:"created_at"`
	UpdatedAt pgtype.Timestamptz `db:"updated_at"`
	DeletedAt pgtype.Timestamptz `db:"deleted_at"`
}

func (u *User) Tablename() string {
	return "users"
}

func (u *User) ColumnsName() []string {
	return []string{"id", "name", "email", "password", "role", "created_at", "updated_at", "deleted_at"}
}

func (u *User) ToRow() []any {
	return []any{u.ID, u.Name, u.Email, u.Password, u.Role, u.CreatedAt, u.UpdatedAt, u.DeletedAt}
}

type UserRequest struct {
	UserId   string
	Name     string
	Email    string
	Password string
	Role     string
}

type UserResponse struct {
	UserId    string
	Name      string
	Email     string
	Password  string
	Role      string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListRequest struct {
	PageSize  uint64
	PageToken string
	Role      string
	Email     string
	Name      string
}
