package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sky-takeout/microservices/goodsService/common"
	"sky-takeout/microservices/goodsService/internal/handler"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/controller"
	"sky-takeout/microservices/goodsService/internal/repository/dao"
	"sky-takeout/microservices/goodsService/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	resources := common.MustInitForService()
	defer func() {
		if err := resources.Close(); err != nil {
			log.Printf("goodsService close resources error: %v", err)
		}
	}()

	r := gin.Default()
	r.GET("/healthz", handler.Health)

	api := r.Group("/goods")
	dishCtrl := controller.NewDishController(
		service.NewDishService(dao.NewDishDao(global.DB)),
	)
	dishCtrl.InitApiRouter(api)

	addr := ":18083"
	server := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("goodsService listening on %s (gin mode)", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("goodsService serve error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("goodsService shutdown error: %v", err)
	}
}
