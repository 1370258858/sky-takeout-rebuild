package main

import (
	"context"
	"fmt"
	"sky-takeout/microservices/deliveryService/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 列表配送
func NewListDeliveriesToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	_ = req

	var orderId *uint64
	if orderIdVal, ok := input["orderId"]; ok {
		switch v := orderIdVal.(type) {
		case float64:
			id := uint64(v)
			orderId = &id
		case int:
			id := uint64(v)
			orderId = &id
		case string:
			var id uint64
			fmt.Sscanf(v, "%d", &id)
			orderId = &id
		}
	}

	var status *int
	if statusVal, ok := input["status"]; ok {
		switch v := statusVal.(type) {
		case float64:
			s := int(v)
			status = &s
		case int:
			status = &v
		}
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
	orderId, ok := input["orderId"]
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: orderId is required")
	}

	var oid uint64
	switch v := orderId.(type) {
	case float64:
		oid = uint64(v)
	case int:
		oid = uint64(v)
	case string:
		fmt.Sscanf(v, "%d", &oid)
	default:
		return nil, nil, fmt.Errorf("invalid orderId type")
	}

	if oid == 0 {
		return nil, nil, fmt.Errorf("invalid request: orderId must be greater than 0")
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
	orderId, ok := input["orderId"]
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: orderId is required")
	}

	var oid uint64
	switch v := orderId.(type) {
	case float64:
		oid = uint64(v)
	case int:
		oid = uint64(v)
	case string:
		fmt.Sscanf(v, "%d", &oid)
	default:
		return nil, nil, fmt.Errorf("invalid orderId type")
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
	orderId, ok := input["orderId"]
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: orderId is required")
	}

	var oid uint64
	switch v := orderId.(type) {
	case float64:
		oid = uint64(v)
	case int:
		oid = uint64(v)
	case string:
		fmt.Sscanf(v, "%d", &oid)
	default:
		return nil, nil, fmt.Errorf("invalid orderId type")
	}

	status, ok := input["status"]
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: status is required")
	}

	var st int
	switch v := status.(type) {
	case float64:
		st = int(v)
	case int:
		st = v
	default:
		return nil, nil, fmt.Errorf("invalid status type")
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
	orderId, ok := input["orderId"]
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: orderId is required")
	}

	var oid uint64
	switch v := orderId.(type) {
	case float64:
		oid = uint64(v)
	case int:
		oid = uint64(v)
	case string:
		fmt.Sscanf(v, "%d", &oid)
	default:
		return nil, nil, fmt.Errorf("invalid orderId type")
	}

	address, ok := input["address"]
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: address is required")
	}

	addressStr, ok := address.(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid address type")
	}

	updateReq := &model.UpdateAddressRequest{DeliveryAddress: addressStr}
	delivery, err := DeliveryCtrl.Service.UpdateAddressByOrderID(ctx, oid, updateReq)
	if err != nil {
		return nil, nil, err
	}
	return nil, delivery, nil
}
