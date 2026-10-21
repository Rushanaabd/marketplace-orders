package main

import (
	"log"

	gen "marketplace-orders/internal/api/gen"
	"marketplace-orders/internal/db"
	"marketplace-orders/internal/handler"
	"marketplace-orders/internal/middleware"
	"marketplace-orders/internal/repository"
	"marketplace-orders/internal/service"
	"marketplace-orders/internal/util"

	"github.com/gin-gonic/gin"
)

func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	productRepo := repository.NewProductRepo(database)
	orderRepo := repository.NewOrderRepo(database)
	promoRepo := repository.NewPromoRepo(database)

	orderSvc := service.NewOrderService(orderRepo, promoRepo)
	h := handler.New(productRepo, database)
	h.SetService(orderSvc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger())

	gen.RegisterHandlersWithOptions(r, h, gen.GinServerOptions{
		// единый формат ошибок при биндинге параметров
		ErrorHandler: func(c *gin.Context, err error, statusCode int) {
			util.Error(c, statusCode, "VALIDATION_ERROR", err.Error(), nil)
		},
	})

	log.Println("starting server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
