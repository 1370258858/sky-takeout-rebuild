package service

import (
	"context"
	"fmt"
	"log"
	"sky-takeout/microservices/goodsService/internal/model"
	"sky-takeout/microservices/goodsService/internal/repository/dao"
)

type DishService interface {
	List(ctx context.Context, req *model.Resquest) ([]model.Dish, error)
	GetByID(ctx context.Context, id uint64) (*model.Dish, error)
}

type DishServiceImpl struct {
	repo *dao.DishDao
}

func (d *DishServiceImpl) List(ctx context.Context, req *model.Resquest) ([]model.Dish, error) {
	if req == nil {
		log.Printf("[SVC][goods][ERR] list failed nil request")
		return nil, fmt.Errorf("nil request")
	}
	// 这里的service层主要负责处理业务逻辑，调用repository层进行数据访问，最后返回结果给controller层
	log.Printf("[SVC][goods] list request id=%d", req.ID)
	dish, err := d.repo.List(ctx, *req)
	if err != nil {
		log.Printf("[SVC][goods][ERR] list failed id=%d err=%v", req.ID, err)
		return nil, err
	}
	return dish, err

}

func (d *DishServiceImpl) GetByID(ctx context.Context, id uint64) (*model.Dish, error) {
	log.Printf("[SVC][goods] get-by-id request goodId=%d", id)
	item, err := d.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][goods][ERR] get-by-id failed goodId=%d err=%v", id, err)
		return nil, err
	}
	return item, nil
}

// 理解gin gorm的核心 函数签名返回值为interface 但是实际返回值为具体的结构体类型 通过接口实现了多态 使得代码更加灵活和可扩展
func NewDishService(repo *dao.DishDao) DishService {
	return &DishServiceImpl{repo: repo}
}
