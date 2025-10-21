package db

import (
	"github.com/julywind168/flying/demo/common/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func init() {
	dsn := "host=localhost user=postgres dbname=game port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}
	err = db.AutoMigrate(&model.User{})
	if err != nil {
		panic("failed to migrate table: " + err.Error())
	}
	DB = db
}
