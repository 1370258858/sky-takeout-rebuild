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

	orderrpcserver "sky-takeout/microservices/orderService/internal/rpc/server"

	"sky-takeout/microservices/orderService/common"
	"sky-takeout/microservices/orderService/global"
	"sky-takeout/microservices/orderService/internal/controller"
	"sky-takeout/microservices/orderService/internal/repository/dao"
	goodsclient "sky-takeout/microservices/orderService/internal/rpc/client"
	"sky-takeout/microservices/orderService/internal/service"
	orderrpcv1 "sky-takeout/microservices/rpc/pb/orderv1"

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

	repo := dao.NewOrderDao(global.DB)
	api := r.Group("/order")
	orderCtrl := controller.NewOrderController(
		service.NewOrderService(repo),
	)
	orderCtrl.InitApiRouter(api)

	addr := ":18084"
	server := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Printf("orderService listening on %s (gin mode)", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("orderService serve error: %v", err)
		}
	}()

	// Initialize gRPC server.
	grpcAddr := os.Getenv("ORDER_SERVICE_GRPC_ADDR")
	if strings.TrimSpace(grpcAddr) == "" {
		grpcAddr = ":19083"
	}
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("orderService listen grpc error: %v", err)
	}

	// 这里注册orderService的grpc服务
	grpcServer := grpc.NewServer()
	orderrpcv1.RegisterOrderServer(grpcServer, orderrpcserver.NewOrderRPCServer())

	go func() {
		log.Printf("orderService grpc listening on %s", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("orderService grpc serve error: %v", err)
		}
	}()
	// 这里注册goodsService的grpc客户端，方便orderService调用goodsService的grpc接口
	goodsRPCAddr := os.Getenv("GOODS_SERVICE_GRPC_ADDR")
	if strings.TrimSpace(goodsRPCAddr) == "" {
		goodsRPCAddr = "localhost:19083"
	}
	conn, err := grpc.Dial(goodsRPCAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("orderService connect to goodsService grpc error: %v", err)
	}
	defer conn.Close()
	global.GoodsRPCClient = goodsclient.NewGoodsRPCClient(conn)
	//监听取消信号，优雅关闭服务
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("orderService shutdown error: %v", err)
	}

}
