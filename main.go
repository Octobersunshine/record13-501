package main

import (
	"log"

	"econtract/handler"
	"econtract/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func initDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("econtract.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func main() {
	db := initDB()

	r := gin.Default()

	h := handler.New(db)
	h.RegisterRoutes(r)

	log.Println("server starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
