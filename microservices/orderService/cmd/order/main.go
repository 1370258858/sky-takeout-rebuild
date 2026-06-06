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

	"sky-takeout/microservices/orderService/common"
	"sky-takeout/microservices/orderService/global"
	"sky-takeout/microservices/orderService/internal/controller"
	"sky-takeout/microservices/orderService/internal/repository/dao"
	rpcclient "sky-takeout/microservices/orderService/internal/rpc/client"
	orderv1 "sky-takeout/microservices/orderService/internal/rpc/pb"
	rpcserver "sky-takeout/microservices/orderService/internal/rpc/server"
	"sky-takeout/microservices/orderService/internal/service"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	resources := common.MustInitForService()
	defer func() {
		if err := resources.Close(); err != nil {
			log.Printf("orderService close resources error: %v", err)
		}
	}()

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"service": "orderService",
			"status":  "ok",
		})
	})

	goodsClient, err := rpcclient.NewGoodsRPCClientFromEnv()
	if err != nil {
		log.Fatalf("orderService connect goodsService grpc error: %v", err)
	}
	defer func() {
		if err := goodsClient.Close(); err != nil {
			log.Printf("orderService close goods grpc conn error: %v", err)
		}
	}()

	r.GET("/order/rpc/goods/by-id", func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.DefaultQuery("id", "1"), 10, 64)
		count, err := goodsClient.ListDishByID(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"user_id":     id,
			"order_count": count,
			"status":      "ok",
		})
	})

	repo := dao.NewOrderDao(global.DB)
	api := r.Group("/order")
	orderCtrl := controller.NewOrderController(
		service.NewOrderService(repo),
	)
	orderCtrl.InitApiRouter(api)

	addr := ":18084"
	server := &http.Server{Addr: addr, Handler: r}

	grpcAddr := os.Getenv("ORDER_SERVICE_GRPC_ADDR")
	if strings.TrimSpace(grpcAddr) == "" {
		grpcAddr = ":19084"
	}
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("orderService listen grpc error: %v", err)
	}
	grpcServer := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcServer, rpcserver.NewOrderRPCServer(goodsClient))

	go func() {
		log.Printf("orderService listening on %s (gin mode)", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("orderService serve error: %v", err)
		}
	}()

	go func() {
		log.Printf("orderService grpc listening on %s", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("orderService grpc serve error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("orderService shutdown error: %v", err)
	}
	grpcServer.GracefulStop()
}
