package server

import (
	"context"
	pb "sky-takeout/microservices/rpc/pb/orderv1"
)

type OrderGrpcServer struct {
	pb.UnimplementedOrderServer
}

func NewOrderRPCServer() *OrderGrpcServer {
	return &OrderGrpcServer{}
}

func (s *OrderGrpcServer) GetOrderById(ctx context.Context, in *pb.GetOrderByIdRequest) (*pb.GetOrderByIdResponse, error) {
	// 这里的service层主要负责处理业务逻辑，调用repository层进行数据访问，最后返回结果给controller层
	return &pb.GetOrderByIdResponse{}, nil
}

func (s *OrderGrpcServer) ListOrders(ctx context.Context, in *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	// 这里的service层主要负责处理业务逻辑，调用repository层进行数据访问，最后返回结果给controller层
	return &pb.ListOrdersResponse{}, nil
}
