package main

import (
	"context"
	"fmt"
	"path/filepath"
	mcpCommon "sky-takeout/microservices/mcpcommonUnit"
	mcptool "sky-takeout/microservices/orderService/common/mcptool"
	"sky-takeout/microservices/orderService/internal/model"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type cartDetailOutput struct {
	Items []model.OrderCart `json:"items"`
}

var (
	toolRulesOnce   sync.Once
	toolUpdateFlags = map[string]bool{}
	toolFactScopes  = map[string][]string{}
)

func loadToolFactRules() {
	toolRulesOnce.Do(func() {
		cfg, err := LoadMCPConfig(filepath.Join(".", "mcp-config.yaml"))
		if err != nil {
			return
		}
		for _, t := range cfg.Tools {
			toolUpdateFlags[t.Name] = t.UpdateFacts
			toolFactScopes[t.Name] = t.FactScopes
		}
	})
}

func buildUpdateFacts(toolName string, data any, input any) []model.UpdateFact {
	loadToolFactRules()
	if !toolUpdateFlags[toolName] {
		return nil
	}
	scopes := toolFactScopes[toolName]
	if len(scopes) == 0 {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	updates := make([]model.UpdateFact, 0, len(scopes))
	for _, scope := range scopes {
		if v, ok := factValueByScope(scope, data, input); ok {
			updates = append(updates, model.UpdateFact{
				Key:        scope,
				Value:      v,
				Confidence: "HIGH",
				Source:     "api",
				UpdatedAt:  now,
			})
		}
	}
	return updates
}

func factValueByScope(scope string, data any, input any) (any, bool) {
	switch scope {
	case "order.id":
		if o, ok := data.(*model.Order); ok && o != nil {
			return o.ID, true
		}
	case "order.status":
		if o, ok := data.(*model.Order); ok && o != nil {
			return o.Status, true
		}
		if m, ok := data.(map[string]any); ok {
			if v, ok := m["status"]; ok {
				return v, true
			}
		}
	case "order.created_at":
		if o, ok := data.(*model.Order); ok && o != nil && o.OrderTime != nil {
			return o.OrderTime.UTC().Format(time.RFC3339), true
		}
	case "order.amount":
		if o, ok := data.(*model.Order); ok && o != nil {
			return o.Amount, true
		}
		if c, ok := data.(*model.OrderCart); ok && c != nil {
			return c.Amount, true
		}
		if req, ok := input.(model.CreateOrderRequest); ok {
			return req.Amount, true
		}
		if req, ok := input.(model.CreateCartRequest); ok {
			return req.Amount, true
		}
		if req, ok := input.(model.UpdateCartRequest); ok {
			return req.Amount, true
		}
	case "order.item_count":
		if c, ok := data.(*model.OrderCart); ok && c != nil {
			return c.Quantity, true
		}
		if req, ok := input.(model.CreateOrderRequest); ok {
			return req.Quantity, true
		}
		if req, ok := input.(model.CreateCartRequest); ok {
			return req.Quantity, true
		}
		if req, ok := input.(model.UpdateCartRequest); ok {
			return req.Quantity, true
		}
	case "delivery.estimated_time":
		if o, ok := data.(*model.Order); ok && o != nil && o.EstimatedDeliveryAt != nil {
			return o.EstimatedDeliveryAt.UTC().Format(time.RFC3339), true
		}
	case "delivery.reserve_time":
		if req, ok := input.(model.CreateOrderRequest); ok && req.EstimatedDeliveryTime != "" {
			return req.EstimatedDeliveryTime, true
		}
	case "budget.max":
		if req, ok := input.(model.CreateOrderRequest); ok {
			return req.Amount, true
		}
		if req, ok := input.(model.CreateCartRequest); ok {
			return req.Amount, true
		}
		if req, ok := input.(model.UpdateCartRequest); ok {
			return req.Amount, true
		}
	case "preference.taste":
		if req, ok := input.(model.CreateCartRequest); ok && req.Flavor != "" {
			return req.Flavor, true
		}
		if req, ok := input.(model.UpdateCartRequest); ok && req.Flavor != "" {
			return req.Flavor, true
		}
	}
	return nil, false
}

func wrapOrderWithFacts(toolName string, orderData *model.Order, input any) *model.OrderWithUpdateFacts {
	if orderData == nil {
		return nil
	}
	return &model.OrderWithUpdateFacts{
		Order:       orderData,
		UpdateFacts: buildUpdateFacts(toolName, orderData, input),
	}
}

func wrapCartWithFacts(toolName string, cartData *model.OrderCart, input any) *model.OrderCartWithUpdateFacts {
	if cartData == nil {
		return nil
	}
	return &model.OrderCartWithUpdateFacts{
		OrderCart:   cartData,
		UpdateFacts: buildUpdateFacts(toolName, cartData, input),
	}
}

func attachUpdateFactsMap(toolName string, payload map[string]any, input any) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	if updates := buildUpdateFacts(toolName, payload, input); len(updates) > 0 {
		payload["updateFacts"] = updates
	}
	return payload
}

// to test user id Elicit function
func NewCreateOrderToolHandler(ctx context.Context, req *mcp.CallToolRequest, input model.CreateOrderRequest) (*mcp.CallToolResult, *model.OrderWithUpdateFacts, error) {
	// req.Session.Elicit()
	uid := input.UserID
	if uid <= 0 {
		result, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
			Message: "mcp 的 Elicit模块需要你提供USERID",
			RequestedSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"userId": map[string]any{
						"type":        "integer",
						"description": "用户ID（大于0的整数）",
					},
				},
				"required": []string{"userId"},
			},
		})
		if err != nil {
			return nil, nil, err
		}
		if result.Action != "accept" || result.Content == nil {
			return nil, nil, fmt.Errorf("operation cancelled by user")
		}

		uid, err = mcptool.MustUint64(result.Content, "userId", "user_id")
		if err != nil {
			return nil, nil, fmt.Errorf("invalid elicitation result: %w", err)
		}
	}

	createOrderRequest := model.CreateOrderRequest{
		GoodIDs:       input.GoodIDs,
		Quantity:      input.Quantity,
		UserID:        uid,
		AddressBookID: input.AddressBookID,
		Amount:        input.Amount,
	}
	orderData, err := orderCtrl.Service.Create(ctx, &createOrderRequest)
	if err != nil {
		return nil, nil, err
	}
	return nil, wrapOrderWithFacts("create_order", orderData, input), nil
}

// / 新增购物车商品
func NewCreateCartToolHandler(ctx context.Context, req *mcp.CallToolRequest, input model.CreateCartRequest) (*mcp.CallToolResult, *model.OrderCartWithUpdateFacts, error) {
	_ = req
	if input.UserID == 0 || len(input.GoodIDs) == 0 || input.Amount <= 0 {
		return nil, nil, fmt.Errorf("invalid request: userId/goodIds/amount are required")
	}

	qty := input.Quantity
	if qty <= 0 {
		qty = 1
	}

	createCartRequest := model.CreateCartRequest{
		GoodIDs:  input.GoodIDs,
		Quantity: qty,
		UserID:   input.UserID,
		Amount:   input.Amount,
	}
	cartData, err := orderCtrl.Service.CreateCart(ctx, &createCartRequest)
	if err != nil {
		return nil, nil, err
	}
	return nil, wrapCartWithFacts("create_cart", cartData, input), nil
}

// 查看购物车
func NewCartDetailToolHandler(ctx context.Context, req *mcp.CallToolRequest, input model.CartDetailRequest) (*mcp.CallToolResult, *cartDetailOutput, error) {
	_ = req
	if input.UserID == 0 {
		return nil, &cartDetailOutput{Items: []model.OrderCart{}}, fmt.Errorf("invalid request: userId is required")
	}

	cartData, err := orderCtrl.Service.GetCart(ctx, input.UserID)
	if err != nil {
		return nil, nil, err
	}
	return nil, &cartDetailOutput{Items: cartData}, nil
}

// 修改购物车
func NewUpdateCartToolHandler(ctx context.Context, req *mcp.CallToolRequest, input model.UpdateCartRequest) (*mcp.CallToolResult, *model.OrderCartWithUpdateFacts, error) {
	_ = req
	if input.UserID == 0 || input.CartID == 0 || input.Quantity <= 0 {
		return nil, nil, fmt.Errorf("invalid request: userId/cartId/quantity are required")
	}

	updateCartRequest := model.UpdateCartRequest{
		CartID:   input.CartID,
		Quantity: input.Quantity,
		Flavor:   input.Flavor,
		Amount:   input.Amount,
	}
	cartData, err := orderCtrl.Service.UpdateCart(ctx, input.UserID, &updateCartRequest)
	if err != nil {
		return nil, nil, err
	}
	return nil, wrapCartWithFacts("update_cart", cartData, input), nil
}

// 删除购物车
func NewDeleteCartToolHandler(ctx context.Context, req *mcp.CallToolRequest, input model.DeleteCartRequest) (*mcp.CallToolResult, map[string]any, error) {
	_ = req
	if input.UserID == 0 || input.CartID == 0 {
		return nil, map[string]any{}, fmt.Errorf("invalid request: userId/cartId are required")
	}

	deleteCartRequest := model.DeleteCartRequest{
		UserID: input.UserID,
		CartID: input.CartID,
	}
	err := orderCtrl.Service.DeleteCart(ctx, input.UserID, &deleteCartRequest)
	if err != nil {
		return nil, nil, err
	}
	return nil, attachUpdateFactsMap("delete_cart", map[string]any{"deleted": true, "cartId": input.CartID, "userId": input.UserID}, input), nil
}

// 退款订单
func NewRefundOrderToolHandler(ctx context.Context, req *mcp.CallToolRequest, input model.RefundOrderRequest) (*mcp.CallToolResult, map[string]any, error) {
	_ = req
	if input.OrderID == 0 {
		return nil, map[string]any{}, fmt.Errorf("invalid request: orderId is required")
	}

	refundOrderRequest := model.RefundOrderRequest{
		OrderID: input.OrderID,
	}
	_, err := orderCtrl.Service.Refund(ctx, input.OrderID, &refundOrderRequest)
	if err != nil {
		return nil, nil, err
	}

	return nil, attachUpdateFactsMap("refund_order", map[string]any{"refunded": true, "orderId": input.OrderID}, input), nil
}

// 获取订单详情
func NewDetailToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.OrderWithUpdateFacts, error) {
	_ = req
	id, err := requireUint64Param(input, "id")
	if err != nil {
		return nil, nil, err
	}

	order, err := orderCtrl.Service.Detail(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	return nil, wrapOrderWithFacts("get_order_detail", order, input), nil
}

// 取消订单
func NewCancelOrderToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.OrderWithUpdateFacts, error) {
	_ = req
	id, err := requireUint64Param(input, "id")
	if err != nil {
		return nil, nil, err
	}

	reason := ""
	if r, ok, err := mcpCommon.GetStringParam(input, "reason"); err != nil {
		return nil, nil, fmt.Errorf("invalid request: %w", err)
	} else if ok {
		reason = r
	}

	cancelReq := &model.CancelOrderRequest{Reason: reason}
	order, err := orderCtrl.Service.Cancel(ctx, id, cancelReq)
	if err != nil {
		return nil, nil, err
	}

	return nil, wrapOrderWithFacts("cancel_order", order, input), nil
}

// 支付订单
func NewPayOrderToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.OrderWithUpdateFacts, error) {
	_ = req
	id, err := requireUint64Param(input, "id")
	if err != nil {
		return nil, nil, err
	}

	payStatus := 1 // default to success
	if ps, ok, err := mcpCommon.GetIntParam(input, "payStatus"); err != nil {
		return nil, nil, fmt.Errorf("invalid request: %w", err)
	} else if ok {
		payStatus = ps
	}

	payReq := &model.PayOrderRequest{PayStatus: payStatus}
	order, err := orderCtrl.Service.Pay(ctx, id, payReq)
	if err != nil {
		return nil, nil, err
	}

	return nil, wrapOrderWithFacts("pay_order", order, input), nil
}

// 列表订单
func NewListOrdersToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	_ = req
	uid, err := requireUint64Param(input, "userId")
	if err != nil {
		return nil, map[string]any{}, err
	}

	var status int
	if s, ok, err := mcpCommon.GetIntParam(input, "status"); err != nil {
		return nil, map[string]any{}, fmt.Errorf("invalid request: %w", err)
	} else if ok {
		status = s
	}

	listReq := &model.Request{
		UserID: uid,
		Status: status,
	}
	orders, err := orderCtrl.Service.List(ctx, listReq)
	if err != nil {
		return nil, map[string]any{}, err
	}

	return nil, attachUpdateFactsMap("list_orders", map[string]any{"orders": orders, "count": len(orders)}, input), nil
}

func requireUint64Param(input map[string]any, key string, aliases ...string) (uint64, error) {
	v, ok, err := mcpCommon.GetUint64Param(input, key, aliases...)
	if err != nil {
		return 0, fmt.Errorf("invalid request: %w", err)
	}
	if !ok {
		return 0, fmt.Errorf("invalid request: %s is required", key)
	}
	if v == 0 {
		return 0, fmt.Errorf("invalid request: %s must be greater than 0", key)
	}
	return v, nil
}
