package dao

import (
	"context"
	"sky-takeout/microservices/orderService/common/e"
	"sky-takeout/microservices/orderService/common/retcode"
	"sky-takeout/microservices/orderService/internal/model"

	"gorm.io/gorm"
)

type OrderDao struct {
	db *gorm.DB
}

func NewOrderDao(db *gorm.DB) *OrderDao {
	return &OrderDao{db: db}
}

func (d *OrderDao) List(ctx context.Context, req model.Request) ([]model.Order, error) {
	var orders []model.Order
	query := d.db.WithContext(ctx).Model(&model.Order{})
	if req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if err := query.Order("id desc").Find(&orders).Error; err != nil {
		return nil, retcode.NewError(e.MysqlERR, "list order failed")
	}
	return orders, nil
}
