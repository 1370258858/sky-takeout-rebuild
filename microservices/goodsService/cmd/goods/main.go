package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"sky-takeout/microservices/goodsService/common"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/controller"
	"sky-takeout/microservices/goodsService/internal/handler"
	"sky-takeout/microservices/goodsService/internal/repository/dao"
	rpcclient "sky-takeout/microservices/goodsService/internal/rpc/client"
	goodsv1 "sky-takeout/microservices/goodsService/internal/rpc/pb"
	rpcserver "sky-takeout/microservices/goodsService/internal/rpc/server"
	"sky-takeout/microservices/goodsService/internal/service"

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

	r := gin.Default()
	r.GET("/healthz", handler.Health)

	orderClient, err := rpcclient.NewOrderRPCClientFromEnv()
	if err != nil {
		log.Fatalf("goodsService connect orderService grpc error: %v", err)
	}
	defer func() {
		if err := orderClient.Close(); err != nil {
			log.Printf("goodsService close order grpc conn error: %v", err)
		}
	}()

	r.GET("/goods/rpc/order/summary", func(c *gin.Context) {
		userID, _ := strconv.ParseInt(c.DefaultQuery("userId", "0"), 10, 64)
		resp, err := orderClient.GetOrderSummary(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	})

	api := r.Group("/goods")
	dishCtrl := controller.NewDishController(
		service.NewDishService(dao.NewDishDao(global.DB)),
	)
	dishCtrl.InitApiRouter(api)

	addr := ":18083"
	server := &http.Server{Addr: addr, Handler: r}

	grpcAddr := os.Getenv("GOODS_SERVICE_GRPC_ADDR")
	if strings.TrimSpace(grpcAddr) == "" {
		grpcAddr = ":19083"
	}
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("goodsService listen grpc error: %v", err)
	}
	grpcServer := grpc.NewServer()
	goodsv1.RegisterGoodsServiceServer(grpcServer, rpcserver.NewGoodsRPCServer())

	go func() {
		log.Printf("goodsService listening on %s (gin mode)", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("goodsService serve error: %v", err)
		}
	}()

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
