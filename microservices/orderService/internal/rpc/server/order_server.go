package server

import (
	"context"

	rpcclient "sky-takeout/microservices/orderService/internal/rpc/client"
	orderv1 "sky-takeout/microservices/orderService/internal/rpc/pb"
)

// OrderRPCServer implements order gRPC methods for downstream callers.
type OrderRPCServer struct {
	orderv1.UnimplementedOrderServiceServer
	goodsClient *rpcclient.GoodsRPCClient
}

func NewOrderRPCServer(goodsClient *rpcclient.GoodsRPCClient) *OrderRPCServer {
	return &OrderRPCServer{goodsClient: goodsClient}
}

func (s *OrderRPCServer) GetGoodById(ctx context.Context, req *orderv1.GetGoodByIdRequest) (*orderv1.GetGoodByIdResponse, error) {
	count, err := s.goodsClient.ListDishByID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	return &orderv1.GetGoodByIdResponse{
		UserId:     req.GetUserId(),
		OrderCount: count,
		Status:     "ok",
	}, nil
}
