package server

import (
	"context"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/model"
	pb "sky-takeout/microservices/rpc/pb/goodsv1"
)

type GoodGrpcServer struct {
	pb.UnimplementedGoodsServer
}

func NewGoodsRPCServer() *GoodGrpcServer {
	return &GoodGrpcServer{}
}

func (s *GoodGrpcServer) GetGoodById(ctx context.Context, in *pb.GetGoodByIdRequest) (*pb.GetGoodByIdResponse, error) {
	var good *model.Dish
	global.DB.Limit(1).Find(&good)
	return &pb.GetGoodByIdResponse{
		Id:          in.Id,
		Name:        good.Name,
		Description: good.Description,
		Price:       good.Price,
	}, nil
}
