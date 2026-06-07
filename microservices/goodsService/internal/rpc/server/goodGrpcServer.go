package server

import (
	"context"
	"log"
	"sky-takeout/microservices/goodsService/internal/service"
	pb "sky-takeout/microservices/rpc/pb/goodsv1"
)

type GoodGrpcServer struct {
	pb.UnimplementedGoodsServer
	service service.DishService
}

func NewGoodsRPCServer(service service.DishService) *GoodGrpcServer {
	return &GoodGrpcServer{service: service}
}

func (s *GoodGrpcServer) GetGoodById(ctx context.Context, in *pb.GetGoodByIdRequest) (*pb.GetGoodByIdResponse, error) {
	log.Printf("[RPC][order/delivery->goods] method=GetGoodById req={id:%d}", in.GetId())
	good, err := s.service.GetByID(ctx, uint64(in.GetId()))
	if err != nil || good == nil {
		log.Printf("[RPC][order/delivery->goods] method=GetGoodById resp=empty err=%v", err)
		return &pb.GetGoodByIdResponse{}, err
	}
	flavors := make([]*pb.DishFlavor, 0, len(good.Flavors))
	for i := range good.Flavors {
		f := good.Flavors[i]
		flavors = append(flavors, &pb.DishFlavor{Id: int64(f.Id), DishId: int64(f.DishId), Name: f.Name, Value: f.Value})
	}
	resp := &pb.GetGoodByIdResponse{
		Id:          int64(good.Id),
		Name:        good.Name,
		DishId:      int64(good.DishId),
		Description: good.Description,
		Price:       good.Price,
		Image:       good.Image,
		Status:      int32(good.Status),
		CreateTime:  good.CreateTime.Unix(),
		UpdateTime:  good.UpdateTime.Unix(),
		CreateUser:  int64(good.CreateUser),
		UpdateUser:  int64(good.UpdateUser),
		Flavors:     flavors,
	}
	log.Printf("[RPC][order/delivery->goods] method=GetGoodById resp={id:%d,name:%s,status:%d}", resp.GetId(), resp.GetName(), resp.GetStatus())
	return resp, nil
}
