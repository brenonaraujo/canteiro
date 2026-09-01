package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/brenonaraujo/canteiro/backend/internal/domain/account"
)

// PGAccounts persists accounts in Postgres.
type PGAccounts struct {
	DB *gorm.DB
}

type accountRow struct {
	DeactivatedAt *time.Time `gorm:"column:deactivated_at"`
	ID            string     `gorm:"column:id;primaryKey"`
	GoogleSubject string     `gorm:"column:google_subject"`
	DisplayName   string     `gorm:"column:display_name"`
	Phone         string     `gorm:"column:phone"`
	Status        string     `gorm:"column:status"`
}

func (accountRow) TableName() string { return "accounts" }

// GetByID loads an account by id.
func (p PGAccounts) GetByID(ctx context.Context, id string) (account.Account, error) {
	var row accountRow
	err := p.DB.WithContext(ctx).Where("id = ?", id).First(&row).Error
	return mapAccount(row, err)
}

// GetByGoogleSubject loads the unique account for a Google subject.
func (p PGAccounts) GetByGoogleSubject(ctx context.Context, subject string) (account.Account, error) {
	var row accountRow
	err := p.DB.WithContext(ctx).Where("google_subject = ?", subject).First(&row).Error
	return mapAccount(row, err)
}

// Create inserts a new account. Duplicate Google subject returns ErrDuplicateGoogle.
func (p PGAccounts) Create(ctx context.Context, acc account.Account) error {
	row := toRow(acc)
	if err := p.DB.WithContext(ctx).Create(&row).Error; err != nil {
		if isUnique(err) {
			return account.ErrDuplicateGoogle
		}
		return err
	}
	return nil
}

// Update persists profile and status. Unknown ids return ErrNotFound.
func (p PGAccounts) Update(ctx context.Context, acc account.Account) error {
	res := p.DB.WithContext(ctx).Model(&accountRow{}).Where("id = ?", acc.ID).Updates(map[string]any{
		"display_name":   acc.DisplayName,
		"phone":          acc.Phone,
		"status":         string(acc.Status),
		"deactivated_at": acc.DeactivatedAt,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return account.ErrNotFound
	}
	return nil
}

func mapAccount(row accountRow, err error) (account.Account, error) {
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return account.Account{}, account.ErrNotFound
		}
		return account.Account{}, err
	}
	return account.Account{
		ID:            row.ID,
		GoogleSubject: row.GoogleSubject,
		DisplayName:   row.DisplayName,
		Phone:         row.Phone,
		Status:        account.Status(row.Status),
		DeactivatedAt: row.DeactivatedAt,
	}, nil
}

func toRow(acc account.Account) accountRow {
	return accountRow{
		ID:            acc.ID,
		GoogleSubject: acc.GoogleSubject,
		DisplayName:   acc.DisplayName,
		Phone:         acc.Phone,
		Status:        string(acc.Status),
		DeactivatedAt: acc.DeactivatedAt,
	}
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(err.Error(), "duplicate key")
}
