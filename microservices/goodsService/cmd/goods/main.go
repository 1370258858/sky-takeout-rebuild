package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"sky-takeout/microservices/goodsService/common"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/controller"
	"sky-takeout/microservices/goodsService/internal/handler"
	"sky-takeout/microservices/goodsService/internal/repository/dao"
	goodsrpcserver "sky-takeout/microservices/goodsService/internal/rpc/server"
	"sky-takeout/microservices/goodsService/internal/service"
	goodsrpcv1 "sky-takeout/microservices/rpc/pb/goodsv1"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	resources := common.MustInitForService()
	defer func() {
		if err := resources.Close(); err != nil {
			log.Printf("goodsService close resources error: %v", err)
		}
	}()

	// Initialize Gin router and HTTP server.
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

	// Initialize gRPC server.
	grpcAddr := os.Getenv("GOODS_SERVICE_GRPC_ADDR")
	if strings.TrimSpace(grpcAddr) == "" {
		grpcAddr = ":19083"
	}
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("goodsService listen grpc error: %v", err)
	}
	grpcServer := grpc.NewServer()
	goodsrpcv1.RegisterGoodsServer(grpcServer, goodsrpcserver.NewGoodsRPCServer())

	go func() {
		log.Printf("goodsService grpc listening on %s", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("goodsService grpc serve error: %v", err)
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
	grpcServer.GracefulStop()
}
