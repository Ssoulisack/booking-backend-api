package migration

import (
	"backend/internal/core/domain"

	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SetupInitialData(db *gorm.DB) error {
	encryptPassword, err := bcrypt.GenerateFromPassword([]byte("123456"), 14)
	if err != nil {
		return err
	}
	// Create users
	users := []domain.User{
		{
			Email:        "thongphetmlv@gmail.com",
			Username:     "mlv007",
			Name:         "mlv 007",
			Verified:     true,
			Status:       true,
			LastActive:   time.Now(),
			PasswordHash: encryptPassword,
			Phone:        "0123456789",
		},
		{
			Email:        "Sounymlv@gmail.com",
			Username:     "souny",
			Name:         "Souny MLV",
			Verified:     true,
			Status:       true,
			LastActive:   time.Now(),
			PasswordHash: encryptPassword,
			Phone:        "0123456789",
		},
	}

	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			return err
		}
	}

	return nil
}
