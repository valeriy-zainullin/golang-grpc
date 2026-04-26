package models

import (
	"errors"

	"gorm.io/gorm"
)

type Like struct {
	PostId uint64 `gorm:"primaryKey"`
	UserId uint64 `gorm:"primaryKey"`
}

func (like *Like) Save(db *gorm.DB) error {
	return db.Save(like).Error
}

func (like *Like) Exists(db *gorm.DB) (bool, error) {
	fetchedLike := Like{}
	result := db.Model(like).First(&fetchedLike)
	if result.Error == nil {
		return true, nil
	} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Does not exist
		return false, nil
	} else {
		return false, result.Error
	}
}

func (like *Like) Delete(db *gorm.DB) error {
	return db.Delete(like).Error
}
