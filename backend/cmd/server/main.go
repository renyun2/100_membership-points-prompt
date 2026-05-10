package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"membershippoints/internal/database"
	"membershippoints/internal/handlers"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	if strings.EqualFold(os.Getenv("GIN_MODE"), "release") {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))
	r.GET("/healthz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.String(503, "db closed")
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.String(503, "db ping failed")
			return
		}
		c.String(200, "ok "+time.Now().Format(time.RFC3339))
	})
	handlers.Register(r, db)
	listen := ":8080"
	if v := strings.TrimSpace(os.Getenv("LISTEN_ADDR")); v != "" {
		listen = v
	}
	log.Printf("listening %s", listen)
	if err := r.Run(listen); err != nil {
		log.Fatal(err)
	}
}
