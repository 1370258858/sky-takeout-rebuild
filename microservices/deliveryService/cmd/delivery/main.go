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

	"sky-takeout/microservices/deliveryService/common"
	"sky-takeout/microservices/deliveryService/global"
	"sky-takeout/microservices/deliveryService/internal/controller"
	"sky-takeout/microservices/deliveryService/internal/repository/dao"
	rpcclient "sky-takeout/microservices/deliveryService/internal/rpc/client"
	rpcserver "sky-takeout/microservices/deliveryService/internal/rpc/server"
	"sky-takeout/microservices/deliveryService/internal/service"
	deliveryrpcv1 "sky-takeout/microservices/rpc/pb/deliveryv1"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	resources := common.MustInitForService()
	defer func() {
		if err := resources.Close(); err != nil {
			log.Printf("deliveryService close resources error: %v", err)
		}
	}()

	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"service": "deliveryService",
			"status":  "ok",
		})
	})

	repo := dao.NewDeliveryDao(global.DB)
	deliverySvc := service.NewDeliveryService(repo)
	deliveryCtrl := controller.NewDeliveryController(deliverySvc)
	api := r.Group("/delivery")
	deliveryCtrl.InitApiRouter(api)

	addr := ":18085"
	server := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("deliveryService listening on %s (gin mode)", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("deliveryService serve error: %v", err)
		}
	}()

	grpcAddr := os.Getenv("DELIVERY_SERVICE_GRPC_ADDR")
	if strings.TrimSpace(grpcAddr) == "" {
		grpcAddr = ":19084"
	}
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("deliveryService listen grpc error: %v", err)
	}

	grpcServer := grpc.NewServer()
	deliveryrpcv1.RegisterDeliveryServer(grpcServer, rpcserver.NewDeliveryRPCServer(deliverySvc))
	go func() {
		log.Printf("deliveryService grpc listening on %s", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("deliveryService grpc serve error: %v", err)
		}
	}()

	goodsRPCAddr := os.Getenv("GOODS_SERVICE_GRPC_ADDR")
	if strings.TrimSpace(goodsRPCAddr) == "" {
		goodsRPCAddr = "goods-service:19083"
	}
	goodsConn, err := grpc.Dial(goodsRPCAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("deliveryService connect to goodsService grpc error: %v", err)
	}
	defer goodsConn.Close()
	global.GoodsRPCClient = rpcclient.NewGoodsRPCClient(goodsConn)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	grpcServer.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("deliveryService shutdown error: %v", err)
	}
}
