package models

import (
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name     string `gorm:"type:varchar(150)" json:"name" validate:"required,min=3,max=150"`
	Email    string `gorm:"uniqueIndex;type:varchar(150)" json:"email" validate:"required,email,min=6,max=150"`
	Password string `gorm:"type:text" json:"-" validate:"required,min=6"`
	Role     string `gorm:"type:varchar(50);default:'user'" json:"role" validate:"oneof=user admin"`
	Status   string `gorm:"type:varchar(50);default:'active'" json:"status" validate:"oneof=active inactive disabled"`
}

func (l User) Validate() error {
	v := validator.New()
	return v.Struct(l)
}
