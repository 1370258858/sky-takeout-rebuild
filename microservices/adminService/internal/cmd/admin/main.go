package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"sky-takeout/microservices/adminService/global"
	"sky-takeout/microservices/adminService/initialize"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	router := initialize.GlobalInit()

	// 设置运行环境
	gin.SetMode(global.Config.Server.Level)

	// MCP FUNCTION
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "admin-tools", Version: "v1.0.0"}, nil)
	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, nil)
	mcpAddr := strings.TrimSpace(os.Getenv("ADMIN_MCP_ADDR"))
	if mcpAddr == "" {
		mcpAddr = ":8001"
	}
	mcpPath := strings.TrimSpace(os.Getenv("ADMIN_MCP_PATH"))
	if mcpPath == "" {
		mcpPath = "/mcp"
	}
	mcpHTTPServer := &http.Server{Addr: mcpAddr, Handler: mcpHandler}
	go func() {
		log.Printf("adminService MCP streamable-http listening on %s (path: %s)", mcpAddr, mcpPath)
		if err := mcpHTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("adminService MCP serve error: %v", err)
		}
	}()

	// Start Gin server
	httpAddr := ":18081"
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: router,
	}

	go func() {
		log.Printf("adminService listening on %s (gin mode)", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("adminService serve error: %v", err)
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("adminService shutdown error: %v", err)
	}
	if err := mcpHTTPServer.Shutdown(ctx); err != nil {
		log.Printf("adminService MCP shutdown error: %v", err)
	}
}
