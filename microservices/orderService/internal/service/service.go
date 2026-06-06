package service

import (
	"context"
	"sky-takeout/microservices/orderService/internal/model"
	"sky-takeout/microservices/orderService/internal/repository/dao"
)

type OrderService interface {
	List(ctx context.Context, req *model.Request) ([]model.Order, error)
}

type OrderServiceImpl struct {
	repo *dao.OrderDao
}

func NewOrderService(repo *dao.OrderDao) OrderService {
	return &OrderServiceImpl{repo: repo}
}

func (s *OrderServiceImpl) List(ctx context.Context, req *model.Request) ([]model.Order, error) {
	return s.repo.List(ctx, *req)
}
