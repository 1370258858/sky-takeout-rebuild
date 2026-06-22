package main

import (
	"context"
	"fmt"
	"sky-takeout/microservices/deliveryService/internal/model"
	mcpCommon "sky-takeout/microservices/mcpcommonUnit"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 列表配送
func NewListDeliveriesToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	_ = req

	var orderId *uint64
	if id, ok, err := mcpCommon.GetUint64Param(input, "orderId"); err != nil {
		return nil, nil, fmt.Errorf("invalid request: %w", err)
	} else if ok {
		orderId = &id
	}

	var status *int
	if s, ok, err := mcpCommon.GetIntParam(input, "status"); err != nil {
		return nil, nil, fmt.Errorf("invalid request: %w", err)
	} else if ok {
		status = &s
	}

	listReq := &model.Request{
		OrderID: 0,
		Status:  status,
	}
	if orderId != nil {
		listReq.OrderID = *orderId
	}

	deliveries, err := DeliveryCtrl.Service.List(ctx, listReq)
	if err != nil {
		return nil, map[string]any{}, err
	}

	return nil, map[string]any{"deliveries": deliveries, "count": len(deliveries)}, nil
}

// 获取配送详情
func NewGetDeliveryDetailToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.Delivery, error) {
	_ = req
	oid, err := requireOrderID(input)
	if err != nil {
		return nil, nil, err
	}

	delivery, err := DeliveryCtrl.Service.GetByOrderID(ctx, oid)
	if err != nil {
		return nil, nil, err
	}
	return nil, delivery, nil
}

// 创建配送
func NewCreateDeliveryToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.Delivery, error) {
	_ = req
	oid, err := requireOrderID(input)
	if err != nil {
		return nil, nil, err
	}

	createReq := &model.CreateDeliveryRequest{
		OrderID:             oid,
		DeliveryNo:          "",
		Status:              1,
		GoodsInfo:           "",
		RiderID:             new(uint64),
		RiderName:           "",
		RiderPhone:          "",
		PickupAddress:       "",
		DeliveryAddress:     "",
		Remark:              "",
		ExpectedArrivalTime: "",
	}

	// Override with input values
	if v, ok := input["deliveryNo"].(string); ok {
		createReq.DeliveryNo = v
	}
	if v, ok := input["status"].(float64); ok {
		createReq.Status = int(v)
	}
	if v, ok := input["goodsInfo"].(string); ok {
		createReq.GoodsInfo = v
	}
	if v, ok := input["riderName"].(string); ok {
		createReq.RiderName = v
	}
	if v, ok := input["riderPhone"].(string); ok {
		createReq.RiderPhone = v
	}
	if v, ok := input["pickupAddress"].(string); ok {
		createReq.PickupAddress = v
	}
	if v, ok := input["deliveryAddress"].(string); ok {
		createReq.DeliveryAddress = v
	}
	if v, ok := input["remark"].(string); ok {
		createReq.Remark = v
	}
	if v, ok := input["expectedArrivalTime"].(string); ok {
		createReq.ExpectedArrivalTime = v
	}

	delivery, err := DeliveryCtrl.Service.Create(ctx, createReq)
	if err != nil {
		return nil, nil, err
	}
	return nil, delivery, nil
}

// 更新配送状态
func NewUpdateDeliveryStatusToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.Delivery, error) {
	_ = req
	oid, err := requireOrderID(input)
	if err != nil {
		return nil, nil, err
	}

	st, ok, err := mcpCommon.GetIntParam(input, "status")
	if err != nil {
		return nil, nil, fmt.Errorf("invalid request: %w", err)
	}
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: status is required")
	}

	updateReq := &model.UpdateStatusRequest{Status: st}
	delivery, err := DeliveryCtrl.Service.UpdateStatusByOrderID(ctx, oid, updateReq)
	if err != nil {
		return nil, nil, err
	}
	return nil, delivery, nil
}

// 更新配送地址
func NewUpdateDeliveryAddressToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.Delivery, error) {
	_ = req
	oid, err := requireOrderID(input)
	if err != nil {
		return nil, nil, err
	}

	addressStr, ok, err := mcpCommon.GetStringParam(input, "address")
	if err != nil {
		return nil, nil, fmt.Errorf("invalid request: %w", err)
	}
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: address is required")
	}

	updateReq := &model.UpdateAddressRequest{DeliveryAddress: addressStr}
	delivery, err := DeliveryCtrl.Service.UpdateAddressByOrderID(ctx, oid, updateReq)
	if err != nil {
		return nil, nil, err
	}
	return nil, delivery, nil
}

func requireOrderID(input map[string]any) (uint64, error) {
	v, ok, err := mcpCommon.GetUint64Param(input, "orderId")
	if err != nil {
		return 0, fmt.Errorf("invalid request: %w", err)
	}
	if !ok {
		return 0, fmt.Errorf("invalid request: orderId is required")
	}
	if v == 0 {
		return 0, fmt.Errorf("invalid request: orderId must be greater than 0")
	}
	return v, nil
}
