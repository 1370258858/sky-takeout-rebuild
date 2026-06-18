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
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
)

var DeliveryCtrl *controller.DeliveryController

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
	DeliveryCtrl = controller.NewDeliveryController(deliverySvc)
	api := r.Group("/delivery")
	DeliveryCtrl.InitApiRouter(api)

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

	// MCP FUNCTION
	// Load MCP configuration from file
	mcpConfig, err := LoadMCPConfig("")
	if err != nil {
		log.Printf("warning: failed to load MCP config, using defaults: %v", err)
		mcpConfig = &MCPConfig{
			Server: MCPServerConfig{
				Name:    "delivery-tools",
				Version: "v1.0.0",
				Address: ":8002",
				Path:    "/mcp",
			},
		}
	}
	if err := mcpConfig.ValidateConfig(); err != nil {
		log.Fatalf("invalid MCP config: %v", err)
	}
	mcpConfig.PrintConfig()

	// Create MCP server with config
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    mcpConfig.Server.Name,
		Version: mcpConfig.Server.Version,
	}, nil)

	// Register all tools from configuration
	for _, toolCfg := range mcpConfig.Tools {
		switch toolCfg.Handler {
		case "ListDeliveries":
			mcp.AddTool(mcpServer, &mcp.Tool{Name: toolCfg.Name, Description: toolCfg.Description}, NewListDeliveriesToolHandler)
		case "GetDeliveryDetail":
			mcp.AddTool(mcpServer, &mcp.Tool{Name: toolCfg.Name, Description: toolCfg.Description}, NewGetDeliveryDetailToolHandler)
		case "CreateDelivery":
			mcp.AddTool(mcpServer, &mcp.Tool{Name: toolCfg.Name, Description: toolCfg.Description}, NewCreateDeliveryToolHandler)
		case "UpdateDeliveryStatus":
			mcp.AddTool(mcpServer, &mcp.Tool{Name: toolCfg.Name, Description: toolCfg.Description}, NewUpdateDeliveryStatusToolHandler)
		case "UpdateDeliveryAddress":
			mcp.AddTool(mcpServer, &mcp.Tool{Name: toolCfg.Name, Description: toolCfg.Description}, NewUpdateDeliveryAddressToolHandler)
		default:
			log.Printf("warning: unknown tool handler %s", toolCfg.Handler)
		}
	}

	PrintAllTools(mcpConfig)

	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, nil)
	mcpAddr := mcpConfig.Server.Address
	path := mcpConfig.Server.Path
	mcpHTTPServer := &http.Server{Addr: mcpAddr, Handler: mcpHandler}
	go func() {
		log.Printf("deliveryService MCP streamable-http listening on %s (path: %s)", mcpAddr, path)
		if err := mcpHTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("deliveryService MCP serve error: %v", err)
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
	if err := mcpHTTPServer.Shutdown(ctx); err != nil {
		log.Printf("deliveryService MCP shutdown error: %v", err)
	}
}
