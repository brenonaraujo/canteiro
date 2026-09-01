package auth

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// PGSessions persists session hashes in Postgres.
type PGSessions struct {
	DB *gorm.DB
}

type sessionRow struct {
	ExpiresAt time.Time `gorm:"column:expires_at"`
	ID        string    `gorm:"column:id;primaryKey"`
	AccountID string    `gorm:"column:account_id"`
	TokenHash string    `gorm:"column:token_hash"`
}

func (sessionRow) TableName() string { return "sessions" }

// Create stores a session hash.
func (p PGSessions) Create(sess Session) error {
	row := sessionRow(sess)
	return p.DB.Create(&row).Error
}

// GetByTokenHash returns a non-expired session.
func (p PGSessions) GetByTokenHash(hash string) (Session, error) {
	var row sessionRow
	err := p.DB.Where("token_hash = ? AND expires_at > ?", hash, time.Now().UTC()).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Session{}, ErrNoSession
		}
		return Session{}, err
	}
	return Session(row), nil
}

// DeleteByTokenHash revokes a session (logout).
func (p PGSessions) DeleteByTokenHash(hash string) error {
	return p.DB.Where("token_hash = ?", hash).Delete(&sessionRow{}).Error
}
