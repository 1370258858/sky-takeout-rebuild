package server

import (
	"context"
	"log"
	"time"

	"sky-takeout/microservices/deliveryService/internal/model"
	"sky-takeout/microservices/deliveryService/internal/service"
	pb "sky-takeout/microservices/rpc/pb/deliveryv1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type DeliveryRPCServer struct {
	pb.UnimplementedDeliveryServer
	service service.DeliveryService
}

func NewDeliveryRPCServer(service service.DeliveryService) *DeliveryRPCServer {
	return &DeliveryRPCServer{service: service}
}

func (s *DeliveryRPCServer) GetDeliveryByOrderId(ctx context.Context, in *pb.GetDeliveryByOrderIdRequest) (*pb.GetDeliveryByOrderIdResponse, error) {
	log.Printf("[RPC][order->delivery] method=GetDeliveryByOrderId req={orderId:%d}", in.GetOrderId())
	delivery, err := s.service.GetByOrderID(ctx, in.GetOrderId())
	if err != nil {
		return nil, err
	}
	log.Printf("[RPC][order->delivery] method=GetDeliveryByOrderId resp={id:%d,status:%d}", delivery.ID, delivery.Status)
	return &pb.GetDeliveryByOrderIdResponse{Delivery: toPBDeliveryDetail(delivery)}, nil
}

func (s *DeliveryRPCServer) ListDeliveries(ctx context.Context, in *pb.ListDeliveriesRequest) (*pb.ListDeliveriesResponse, error) {
	log.Printf("[RPC][gateway/order->delivery] method=ListDeliveries req={orderId:%d,status:%v}", in.GetOrderId(), in.Status)
	var status *int
	if in.Status != nil {
		v := int(in.GetStatus())
		status = &v
	}
	items, err := s.service.List(ctx, &model.Request{OrderID: in.GetOrderId(), Status: status})
	if err != nil {
		return nil, err
	}
	out := make([]*pb.DeliveryDetail, 0, len(items))
	for i := range items {
		item := items[i]
		out = append(out, toPBDeliveryDetail(&item))
	}
	log.Printf("[RPC][gateway/order->delivery] method=ListDeliveries respCount=%d", len(out))
	return &pb.ListDeliveriesResponse{Items: out}, nil
}

func (s *DeliveryRPCServer) CreateDelivery(ctx context.Context, in *pb.CreateDeliveryRequest) (*pb.CreateDeliveryResponse, error) {
	log.Printf("[RPC][order->delivery] method=CreateDelivery req={orderId:%d,deliveryNo:%s,status:%d}", in.GetOrderId(), in.GetDeliveryNo(), in.GetStatus())
	var riderID *uint64
	if in.GetRiderId() > 0 {
		v := in.GetRiderId()
		riderID = &v
	}
	created, err := s.service.Create(ctx, &model.CreateDeliveryRequest{
		OrderID:             in.GetOrderId(),
		DeliveryNo:          in.GetDeliveryNo(),
		Status:              int(in.GetStatus()),
		GoodsInfo:           in.GetGoodsInfo(),
		RiderID:             riderID,
		RiderName:           in.GetRiderName(),
		RiderPhone:          in.GetRiderPhone(),
		PickupAddress:       in.GetPickupAddress(),
		DeliveryAddress:     in.GetDeliveryAddress(),
		ExpectedArrivalTime: in.GetExpectedArrivalTime(),
		Remark:              in.GetRemark(),
	})
	if err != nil {
		return nil, err
	}
	log.Printf("[RPC][order->delivery] method=CreateDelivery resp={id:%d,status:%d}", created.ID, created.Status)
	return &pb.CreateDeliveryResponse{Delivery: toPBDeliveryDetail(created)}, nil
}

func (s *DeliveryRPCServer) UpdateDeliveryStatus(ctx context.Context, in *pb.UpdateDeliveryStatusRequest) (*pb.UpdateDeliveryStatusResponse, error) {
	log.Printf("[RPC][gateway/order->delivery] method=UpdateDeliveryStatus req={orderId:%d,status:%d}", in.GetOrderId(), in.GetStatus())
	var riderID *uint64
	if in.GetRiderId() > 0 {
		v := in.GetRiderId()
		riderID = &v
	}
	updated, err := s.service.UpdateStatusByOrderID(ctx, in.GetOrderId(), &model.UpdateStatusRequest{
		Status:     int(in.GetStatus()),
		Remark:     in.GetRemark(),
		RiderID:    riderID,
		RiderName:  in.GetRiderName(),
		RiderPhone: in.GetRiderPhone(),
	})
	if err != nil {
		return nil, err
	}
	log.Printf("[RPC][gateway/order->delivery] method=UpdateDeliveryStatus resp={id:%d,status:%d}", updated.ID, updated.Status)
	return &pb.UpdateDeliveryStatusResponse{Delivery: toPBDeliveryDetail(updated)}, nil
}

func (s *DeliveryRPCServer) UpdateDeliveryByOrderId(ctx context.Context, in *pb.UpdateDeliveryByOrderIdRequest) (*pb.UpdateDeliveryByOrderIdResponse, error) {
	log.Printf("[RPC][order->delivery] method=UpdateDeliveryByOrderId req={orderId:%d,status:%d}", in.GetOrderId(), in.GetStatus())
	var riderID *uint64
	if in.GetRiderId() > 0 {
		v := in.GetRiderId()
		riderID = &v
	}
	updated, err := s.service.UpdateByOrderIDInsert(ctx, in.GetOrderId(), &model.UpdateStatusRequest{
		Status:     int(in.GetStatus()),
		Remark:     in.GetRemark(),
		RiderID:    riderID,
		RiderName:  in.GetRiderName(),
		RiderPhone: in.GetRiderPhone(),
	})
	if err != nil {
		return nil, err
	}
	log.Printf("[RPC][order->delivery] method=UpdateDeliveryByOrderId resp={id:%d,status:%d}", updated.ID, updated.Status)
	return &pb.UpdateDeliveryByOrderIdResponse{Delivery: toPBDeliveryDetail(updated)}, nil
}

func toPBDeliveryDetail(in *model.Delivery) *pb.DeliveryDetail {
	if in == nil {
		return nil
	}
	riderID := uint64(0)
	if in.RiderID != nil {
		riderID = *in.RiderID
	}
	return &pb.DeliveryDetail{
		Id:                  in.ID,
		OrderId:             in.OrderID,
		DeliveryNo:          in.DeliveryNo,
		Status:              int32(in.Status),
		GoodsInfo:           in.GoodsInfo,
		RiderId:             riderID,
		RiderName:           in.RiderName,
		RiderPhone:          in.RiderPhone,
		PickupAddress:       in.PickupAddress,
		DeliveryAddress:     in.DeliveryAddress,
		DispatchTime:        toTimestamp(in.DispatchTime),
		PickupTime:          toTimestamp(in.PickupTime),
		ExpectedArrivalTime: toTimestamp(in.ExpectedArrivalTime),
		DeliveredTime:       toTimestamp(in.DeliveredTime),
		Remark:              in.Remark,
		CreateTime:          toTimestamp(in.CreateTime),
		UpdateTime:          toTimestamp(in.UpdateTime),
	}
}

func toTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
