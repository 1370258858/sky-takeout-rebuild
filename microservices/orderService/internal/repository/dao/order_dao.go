package dao

import (
	"context"
	"log"
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
	tx := query.Order("id desc").Find(&orders)
	if tx.Error != nil {
		return nil, retcode.NewError(e.MysqlERR, "list order failed")
	}
	log.Printf("[DB][order] query list rows=%d userId=%d status=%v", tx.RowsAffected, req.UserID, req.Status)
	return orders, nil
}

func (d *OrderDao) Create(ctx context.Context, in *model.Order) (*model.Order, error) {
	tx := d.db.WithContext(ctx).Create(in)
	if tx.Error != nil {
		return nil, retcode.NewError(e.MysqlERR, "create order failed")
	}
	log.Printf("[DB][order] insert rows=%d orderId=%d", tx.RowsAffected, in.ID)
	return in, nil
}

func (d *OrderDao) GetByID(ctx context.Context, id uint64) (*model.Order, error) {
	var order model.Order
	tx := d.db.WithContext(ctx).Model(&model.Order{}).Where("id = ?", id).Take(&order)
	if tx.Error != nil {
		if tx.Error == gorm.ErrRecordNotFound {
			return nil, retcode.NewError(e.ErrorOrderNotFound, "order not found")
		}
		return nil, retcode.NewError(e.MysqlERR, "query order failed")
	}
	log.Printf("[DB][order] query by id rows=%d orderId=%d", tx.RowsAffected, id)
	return &order, nil
}

func (d *OrderDao) UpdateByID(ctx context.Context, id uint64, updates map[string]any) error {
	query := d.db.WithContext(ctx).Model(&model.Order{}).Where("id = ?", id)
	if err := query.Updates(updates).Error; err != nil {
		return retcode.NewError(e.MysqlERR, "update order failed")
	}
	if query.RowsAffected == 0 {
		return retcode.NewError(e.ErrorOrderNotFound, "order not found")
	}
	log.Printf("[DB][order] update rows=%d orderId=%d updates=%v", query.RowsAffected, id, updates)
	return nil
}
