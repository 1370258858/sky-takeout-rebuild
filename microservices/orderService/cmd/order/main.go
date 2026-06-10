package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"sky-takeout/microservices/orderService/internal/model"
	"sky-takeout/microservices/orderService/internal/repository/dao"
	goodsclient "sky-takeout/microservices/orderService/internal/rpc/client"
	"sky-takeout/microservices/orderService/internal/service"
	orderrpcv1 "sky-takeout/microservices/rpc/pb/orderv1"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
)

type CreateOrderInput struct {
	UserID        uint64  `json:"userId" jsonschema:"user id"`
	GoodID        uint64  `json:"goodId" jsonschema:"goods id"`
	AddressBookID uint64  `json:"addressBookId" jsonschema:"address book id"`
	Amount        float64 `json:"amount" jsonschema:"order amount"`
	Quantity      int     `json:"quantity,omitempty" jsonschema:"goods quantity"`
}

type CreateOrderOutput struct {
	OrderID      uint64  `json:"orderId" jsonschema:"created order id"`
	OrderNo      string  `json:"orderNo" jsonschema:"created order number"`
	Status       int     `json:"status" jsonschema:"order status"`
	UserID       uint64  `json:"userId" jsonschema:"user id"`
	GoodID       uint64  `json:"goodId" jsonschema:"goods id"`
	Amount       float64 `json:"amount" jsonschema:"order amount"`
	Quantity     int     `json:"quantity" jsonschema:"goods quantity"`
	CreatedAt    string  `json:"createdAt" jsonschema:"order create time"`
	Message      string  `json:"message" jsonschema:"result message"`
	IsMockResult bool    `json:"isMockResult" jsonschema:"whether this result is mocked"`
}

func newCreateOrderToolHandler(orderCtrl *controller.OrderController) func(context.Context, *mcp.CallToolRequest, CreateOrderInput) (*mcp.CallToolResult, *CreateOrderOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateOrderInput) (*mcp.CallToolResult, *CreateOrderOutput, error) {
		_ = req
		if input.UserID == 0 || input.GoodID == 0 || input.AddressBookID == 0 || input.Amount <= 0 {
			return nil, &CreateOrderOutput{}, fmt.Errorf("invalid request: userId/goodId/addressBookId/amount are required")
		}

		qty := input.Quantity
		if qty <= 0 {
			qty = 1
		}

		createOrderRequest := model.CreateOrderRequest{
			GoodID:        int64(input.GoodID),
			Quantity:      qty,
			UserID:        input.UserID,
			AddressBookID: input.AddressBookID,
			Amount:        input.Amount,
		}
		orderData, err := orderCtrl.CreateForMCP(ctx, &createOrderRequest)
		if err != nil {
			return nil, nil, err
		}

		createdAt := ""
		if orderData.OrderTime != nil {
			createdAt = orderData.OrderTime.Format(time.RFC3339)
		}

		result := CreateOrderOutput{
			OrderID:      orderData.ID,
			OrderNo:      orderData.Number,
			Status:       orderData.Status,
			UserID:       orderData.UserID,
			GoodID:       input.GoodID,
			Amount:       orderData.Amount,
			Quantity:     qty,
			CreatedAt:    createdAt,
			Message:      "create_order executed by MCP server",
			IsMockResult: false,
		}

		return nil, &result, nil
	}
}

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
	_, mqCh := resources.MQ()
	const (
		timeoutExchange = "order.pay.timeout.exchange"
		timeoutQueue    = "order.pay.timeout.queue"
		timeoutKey      = "order.pay.timeout"
	)
	if err := mqCh.ExchangeDeclare(timeoutExchange, "x-delayed-message", true, false, false, false, amqp.Table{"x-delayed-type": "direct"}); err != nil {
		log.Fatalf("orderService declare delay exchange error: %v", err)
	}
	if _, err := mqCh.QueueDeclare(timeoutQueue, true, false, false, false, nil); err != nil {
		log.Fatalf("orderService declare delay queue error: %v", err)
	}
	if err := mqCh.QueueBind(timeoutQueue, timeoutKey, timeoutExchange, false, nil); err != nil {
		log.Fatalf("orderService bind delay queue error: %v", err)
	}
	publishTimeout := func(ctx context.Context, orderID uint64) error {
		payload, _ := json.Marshal(model.OrderTimeoutMessage{OrderID: orderID})
		log.Printf("[MQ][order] publish exchange=%s routingKey=%s payload=%s", timeoutExchange, timeoutKey, string(payload))
		err := mqCh.PublishWithContext(ctx, timeoutExchange, timeoutKey, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
			Headers:     amqp.Table{"x-delay": int32(3 * 60 * 1000)},
		})
		if err != nil {
			log.Printf("[MQ][order] publish failed orderId=%d err=%v", orderID, err)
			return err
		}
		log.Printf("[MQ][order] publish success orderId=%d", orderID)
		return nil
	}
	orderSvc := service.NewOrderService(repo, publishTimeout)

	mqConn, _ := resources.MQ()
	consumeCh, err := mqConn.Channel()
	if err != nil {
		log.Fatalf("orderService create consume channel error: %v", err)
	}
	if err = consumeCh.ExchangeDeclare(timeoutExchange, "x-delayed-message", true, false, false, false, amqp.Table{"x-delayed-type": "direct"}); err != nil {
		log.Fatalf("orderService consume declare exchange error: %v", err)
	}
	if _, err = consumeCh.QueueDeclare(timeoutQueue, true, false, false, false, nil); err != nil {
		log.Fatalf("orderService consume declare queue error: %v", err)
	}
	if err = consumeCh.QueueBind(timeoutQueue, timeoutKey, timeoutExchange, false, nil); err != nil {
		log.Fatalf("orderService consume bind queue error: %v", err)
	}
	messages, err := consumeCh.Consume(timeoutQueue, "order-timeout-consumer", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("orderService start timeout consumer error: %v", err)
	}
	go func() {
		for msg := range messages {
			log.Printf("[MQ][order] consume message routingKey=%s body=%s", msg.RoutingKey, string(msg.Body))
			var event model.OrderTimeoutMessage
			if unmarshalErr := json.Unmarshal(msg.Body, &event); unmarshalErr != nil {
				log.Printf("[MQ][order] consume decode failed err=%v", unmarshalErr)
				_ = msg.Nack(false, false)
				continue
			}
			if _, callErr := orderSvc.PayTimeout(context.Background(), event.OrderID); callErr != nil {
				log.Printf("[MQ][order] timeout handler failed orderId=%d err=%v", event.OrderID, callErr)
				_ = msg.Nack(false, true)
				continue
			}
			log.Printf("[MQ][order] timeout handler success orderId=%d", event.OrderID)
			_ = msg.Ack(false)
		}
	}()

	api := r.Group("/order")
	orderCtrl := controller.NewOrderController(
		orderSvc,
	)
	orderCtrl.InitApiRouter(api)

	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "order-tools", Version: "v1.0.0"}, nil)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "create_order", Description: "Create a takeout order with userId, goodId, addressBookId and amount."}, newCreateOrderToolHandler(orderCtrl))
	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, nil)
	mcpAddr := strings.TrimSpace(os.Getenv("ORDER_MCP_ADDR"))
	if mcpAddr == "" {
		mcpAddr = ":8001"
	}
	mcpHTTPServer := &http.Server{Addr: mcpAddr, Handler: mcpHandler}
	go func() {
		log.Printf("orderService MCP streamable-http listening on %s (path: /mcp)", mcpAddr)
		if err := mcpHTTPServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("orderService MCP serve error: %v", err)
		}
	}()

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
		goodsRPCAddr = "goods-service:19083"
	}
	conn, err := grpc.Dial(goodsRPCAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("orderService connect to goodsService grpc error: %v", err)
	}
	log.Printf("[RPC][order->goods] connected addr=%s", goodsRPCAddr)
	defer conn.Close()
	global.GoodsRPCClient = goodsclient.NewGoodsRPCClient(conn)

	deliveryRPCAddr := os.Getenv("DELIVERY_SERVICE_RPC_ADDR")
	if strings.TrimSpace(deliveryRPCAddr) == "" {
		deliveryRPCAddr = "delivery-service:19084"
	}
	deliveryConn, err := grpc.Dial(deliveryRPCAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("orderService connect to deliveryService grpc error: %v", err)
	}
	log.Printf("[RPC][order->delivery] connected addr=%s", deliveryRPCAddr)
	defer deliveryConn.Close()
	global.DeliveryRPCClient = goodsclient.NewDeliveryRPCClient(deliveryConn)
	//监听取消信号，优雅关闭服务
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("orderService shutdown error: %v", err)
	}
	if err := mcpHTTPServer.Shutdown(ctx); err != nil {
		log.Printf("orderService MCP shutdown error: %v", err)
	}
	_ = consumeCh.Close()
	grpcServer.GracefulStop()

}
