package service

import (
	"context"
	"encoding/json"
	"fmt"
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
	UpdateAddressByOrderID(ctx context.Context, orderID uint64, req *model.UpdateAddressRequest) (*model.Delivery, error)
	Review(ctx context.Context, orderID uint64, req *model.ReviewRequest) (*model.Delivery, error)
}

type DeliveryServiceImpl struct {
	repo *dao.DeliveryDao
}

func NewDeliveryService(repo *dao.DeliveryDao) DeliveryService {
	return &DeliveryServiceImpl{repo: repo}
}

func (s *DeliveryServiceImpl) List(ctx context.Context, req *model.Request) ([]model.Delivery, error) {
	if req == nil {
		log.Printf("[SVC][delivery][ERR] list failed nil request")
		return nil, fmt.Errorf("nil request")
	}
	log.Printf("[SVC][delivery] list request orderId=%d status=%v", req.OrderID, req.Status)
	items, err := s.repo.List(ctx, *req)
	if err != nil {
		log.Printf("[SVC][delivery][ERR] list failed orderId=%d status=%v err=%v", req.OrderID, req.Status, err)
		return nil, err
	}
	return items, nil
}

func (s *DeliveryServiceImpl) GetByOrderID(ctx context.Context, orderID uint64) (*model.Delivery, error) {
	log.Printf("[SVC][delivery] detail request orderId=%d", orderID)
	item, err := s.repo.GetByOrderID(ctx, orderID)
	if err != nil {
		log.Printf("[SVC][delivery][ERR] detail failed orderId=%d err=%v", orderID, err)
		return nil, err
	}
	return item, nil
}

func (s *DeliveryServiceImpl) Create(ctx context.Context, req *model.CreateDeliveryRequest) (*model.Delivery, error) {
	if req == nil {
		log.Printf("[SVC][delivery][ERR] create failed nil request")
		return nil, fmt.Errorf("nil request")
	}
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
		} else {
			log.Printf("[SVC][delivery][ERR] create parse expectedArrivalTime failed orderId=%d input=%s err=%v", req.OrderID, req.ExpectedArrivalTime, err)
		}
	}
	created, err := s.repo.Create(ctx, entity)
	if err != nil {
		log.Printf("[SVC][delivery][ERR] create failed orderId=%d deliveryNo=%s err=%v", req.OrderID, req.DeliveryNo, err)
		return nil, err
	}
	return created, nil
}

func (s *DeliveryServiceImpl) UpdateStatusByOrderID(ctx context.Context, orderID uint64, req *model.UpdateStatusRequest) (*model.Delivery, error) {
	if req == nil {
		log.Printf("[SVC][delivery][ERR] update-status failed nil request orderId=%d", orderID)
		return nil, fmt.Errorf("nil request")
	}
	log.Printf("[SVC][delivery] update-status request orderId=%d status=%d", orderID, req.Status)
	updated, err := s.repo.UpdateStatusByOrderID(ctx, orderID, *req)
	if err != nil {
		log.Printf("[SVC][delivery][ERR] update-status failed orderId=%d status=%d err=%v", orderID, req.Status, err)
		return nil, err
	}
	return updated, nil
}

func (s *DeliveryServiceImpl) UpdateByOrderIDInsert(ctx context.Context, orderID uint64, req *model.UpdateStatusRequest) (*model.Delivery, error) {
	if req == nil {
		log.Printf("[SVC][delivery][ERR] update-by-insert failed nil request orderId=%d", orderID)
		return nil, fmt.Errorf("nil request")
	}
	log.Printf("[SVC][delivery] update-by-insert request orderId=%d status=%d", orderID, req.Status)
	updated, err := s.repo.UpdateByOrderIDInsert(ctx, orderID, *req)
	if err != nil {
		log.Printf("[SVC][delivery][ERR] update-by-insert failed orderId=%d status=%d err=%v", orderID, req.Status, err)
		return nil, err
	}
	return updated, nil
}

func (s *DeliveryServiceImpl) UpdateAddressByOrderID(ctx context.Context, orderID uint64, req *model.UpdateAddressRequest) (*model.Delivery, error) {
	if req == nil {
		log.Printf("[SVC][delivery][ERR] update-address failed nil request orderId=%d", orderID)
		return nil, fmt.Errorf("nil request")
	}
	log.Printf("[SVC][delivery] update-address request orderId=%d", orderID)

	history := req.AddressHistory
	if len(history) == 0 && (req.Consignee != "" || req.Phone != "" || req.DeliveryAddress != "") {
		history = append(history, model.AddressHistoryItem{
			Consignee: req.Consignee,
			Phone:     req.Phone,
			Address:   req.DeliveryAddress,
			UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		})
	}
	if len(history) > 0 {
		payload, err := json.Marshal(history)
		if err != nil {
			log.Printf("[SVC][delivery][ERR] update-address marshal history failed orderId=%d err=%v", orderID, err)
			return nil, err
		}
		req.AddressHistoryJSON = string(payload)
	}

	updated, err := s.repo.UpdateAddressByOrderID(ctx, orderID, *req)
	if err != nil {
		log.Printf("[SVC][delivery][ERR] update-address failed orderId=%d err=%v", orderID, err)
		return nil, err
	}
	return updated, nil
}

func (s *DeliveryServiceImpl) Review(ctx context.Context, orderID uint64, req *model.ReviewRequest) (*model.Delivery, error) {
	if req == nil {
		log.Printf("[SVC][delivery][ERR] review failed nil request orderId=%d", orderID)
		return nil, fmt.Errorf("nil request")
	}
	if req.Review == "" {
		return nil, fmt.Errorf("review is required")
	}
	log.Printf("[SVC][delivery] review request orderId=%d", orderID)
	updated, err := s.repo.ReviewByOrderID(ctx, orderID, req.Review)
	if err != nil {
		log.Printf("[SVC][delivery][ERR] review failed orderId=%d err=%v", orderID, err)
		return nil, err
	}
	return updated, nil
}
