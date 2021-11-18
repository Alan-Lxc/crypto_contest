package model

import "gorm.io/gorm"

type Unit struct {
	gorm.Model
	Secretsharenum int64  `gorm:"type:int;not null"'`
	Loglocation    string `gorm:"type:varchar(200);not null"`
}