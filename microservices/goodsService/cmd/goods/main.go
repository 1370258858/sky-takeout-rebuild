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
	"sky-takeout/microservices/goodsService/internal/model"
	"sky-takeout/microservices/goodsService/internal/repository/dao"
	goodsrpcserver "sky-takeout/microservices/goodsService/internal/rpc/server"
	"sky-takeout/microservices/goodsService/internal/service"
	goodsrpcv1 "sky-takeout/microservices/rpc/pb/goodsv1"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
)

var goodsCtrl *controller.DishController

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
	dishService := service.NewDishService(dao.NewDishDao(global.DB))
	goodsCtrl = controller.NewDishController(dishService)
	goodsCtrl.InitApiRouter(api)

	addr := ":18083"
	server := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Printf("goodsService listening on %s (gin mode)", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("goodsService serve error: %v", err)
		}
	}()

	//MCP FUNCTION

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "goods-tools", Version: "v1.0.0"}, nil)
	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "list_goods",
		Description: "List all available goods.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dishes": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
					},
				},
			},
		},
	}, newListGoodsToolHandler)
	mcpAddr := strings.TrimSpace(os.Getenv("GOODS_MCP_ADDR"))
	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	if mcpAddr == "" {
		mcpAddr = ":8001"
	}
	path := strings.TrimSpace(os.Getenv("GOODS_MCP_PATH"))
	if path == "" {
		path = "/mcp"
	}
	mcpHTTPServer := &http.Server{Addr: mcpAddr, Handler: mcpHandler}
	go func() {
		log.Printf("goodsService MCP streamable-http listening on %s (path: %s)", mcpAddr, path)
		if err := mcpHTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("goodsService MCP serve error: %v", err)
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
	goodsrpcv1.RegisterGoodsServer(grpcServer, goodsrpcserver.NewGoodsRPCServer(dishService))

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

func newListGoodsToolHandler(ctx context.Context, req *mcp.CallToolRequest, input model.Resquest) (*mcp.CallToolResult, []model.Dish, error) {
	var result []model.Dish
	dish, err := goodsCtrl.ListMCP(&ctx)
	if err != nil {
		return nil, nil, err
	}
	result = append(result, dish...)

	return nil, result, nil
}
