package service

import (
	"context"
	"log"
	"sky-takeout/microservices/deliveryService/internal/model"
	"sky-takeout/microservices/deliveryService/internal/repository/dao"
	"time"
)

type DeliveryService interface {
	List(ctx context.Context, req *model.Request) ([]model.Delivery, error)
	GetByOrderID(ctx context.Context, orderID uint64) (*model.Delivery, error)
	Create(ctx context.Context, req *model.CreateDeliveryRequest) (*model.Delivery, error)
	UpdateStatusByOrderID(ctx context.Context, orderID uint64, req *model.UpdateStatusRequest) (*model.Delivery, error)
	UpdateByOrderIDInsert(ctx context.Context, orderID uint64, req *model.UpdateStatusRequest) (*model.Delivery, error)
}

type DeliveryServiceImpl struct {
	repo *dao.DeliveryDao
}

func NewDeliveryService(repo *dao.DeliveryDao) DeliveryService {
	return &DeliveryServiceImpl{repo: repo}
}

func (s *DeliveryServiceImpl) List(ctx context.Context, req *model.Request) ([]model.Delivery, error) {
	log.Printf("[SVC][delivery] list request orderId=%d status=%v", req.OrderID, req.Status)
	return s.repo.List(ctx, *req)
}

func (s *DeliveryServiceImpl) GetByOrderID(ctx context.Context, orderID uint64) (*model.Delivery, error) {
	log.Printf("[SVC][delivery] detail request orderId=%d", orderID)
	return s.repo.GetByOrderID(ctx, orderID)
}

func (s *DeliveryServiceImpl) Create(ctx context.Context, req *model.CreateDeliveryRequest) (*model.Delivery, error) {
	log.Printf("[SVC][delivery] create request orderId=%d deliveryNo=%s status=%d", req.OrderID, req.DeliveryNo, req.Status)
	status := req.Status
	if status == 0 {
		status = 1
	}
	now := time.Now()
	entity := &model.Delivery{
		OrderID:         req.OrderID,
		DeliveryNo:      req.DeliveryNo,
		Status:          status,
		GoodsInfo:       req.GoodsInfo,
		RiderID:         req.RiderID,
		RiderName:       req.RiderName,
		RiderPhone:      req.RiderPhone,
		PickupAddress:   req.PickupAddress,
		DeliveryAddress: req.DeliveryAddress,
		Remark:          req.Remark,
		CreateTime:      &now,
		UpdateTime:      &now,
	}
	if req.ExpectedArrivalTime != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", req.ExpectedArrivalTime, time.Local); err == nil {
			entity.ExpectedArrivalTime = &t
		}
	}
	return s.repo.Create(ctx, entity)
}

func (s *DeliveryServiceImpl) UpdateStatusByOrderID(ctx context.Context, orderID uint64, req *model.UpdateStatusRequest) (*model.Delivery, error) {
	log.Printf("[SVC][delivery] update-status request orderId=%d status=%d", orderID, req.Status)
	return s.repo.UpdateStatusByOrderID(ctx, orderID, *req)
}

func (s *DeliveryServiceImpl) UpdateByOrderIDInsert(ctx context.Context, orderID uint64, req *model.UpdateStatusRequest) (*model.Delivery, error) {
	log.Printf("[SVC][delivery] update-by-insert request orderId=%d status=%d", orderID, req.Status)
	return s.repo.UpdateByOrderIDInsert(ctx, orderID, *req)
}
