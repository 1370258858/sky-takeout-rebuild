package main

import (
	"context"
	"fmt"
	"sky-takeout/microservices/goodsService/internal/model"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 列表商品
func NewListGoodsToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, map[string]any, error) {
	_ = req
	_ = input

	dishes, err := goodsCtrl.Service.List(ctx)
	if err != nil {
		return nil, map[string]any{}, err
	}

	return nil, map[string]any{"dishes": dishes, "count": len(dishes)}, nil
}

// 获取商品详情
func NewGetGoodDetailToolHandler(ctx context.Context, req *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, *model.Dish, error) {
	_ = req
	id, ok := input["id"]
	if !ok {
		return nil, nil, fmt.Errorf("invalid request: id is required")
	}

	var dishId uint64
	switch v := id.(type) {
	case float64:
		dishId = uint64(v)
	case int:
		dishId = uint64(v)
	case string:
		fmt.Sscanf(v, "%d", &dishId)
	default:
		return nil, nil, fmt.Errorf("invalid id type")
	}

	if dishId == 0 {
		return nil, nil, fmt.Errorf("invalid request: id must be greater than 0")
	}

	dish, err := goodsCtrl.Service.GetByID(ctx, dishId)
	if err != nil {
		return nil, nil, err
	}
	return nil, dish, nil
}
