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

func (d *OrderDao) CreateOrderCart(ctx context.Context, in *model.OrderCart) (*model.OrderCart, error) {
	tx := d.db.WithContext(ctx).Create(in)
	if tx.Error != nil {
		return nil, retcode.NewError(e.MysqlERR, "create order cart failed")
	}
	log.Printf("[DB][order] insert rows=%d orderCartId=%d", tx.RowsAffected, in.ID)
	return in, nil

}

func (d *OrderDao) GetCartByUserID(ctx context.Context, userID uint64) ([]model.OrderCart, error) {
	var carts []model.OrderCart
	tx := d.db.WithContext(ctx).Model(&model.OrderCart{}).Where("user_id = ?", userID).Order("id desc").Find(&carts)
	if tx.Error != nil {
		return nil, retcode.NewError(e.MysqlERR, "query order cart failed")
	}
	log.Printf("[DB][order] query cart by userId rows=%d userId=%d", tx.RowsAffected, userID)
	return carts, nil
}

func (d *OrderDao) GetCartItemByID(ctx context.Context, userID uint64, cartID uint64) (*model.OrderCart, error) {
	var cart model.OrderCart
	tx := d.db.WithContext(ctx).Model(&model.OrderCart{}).Where("user_id = ? AND id = ?", userID, cartID).Take(&cart)
	if tx.Error != nil {
		if tx.Error == gorm.ErrRecordNotFound {
			return nil, retcode.NewError(e.ErrorOrderCartNotFound, "order cart not found")
		}
		return nil, retcode.NewError(e.MysqlERR, "query order cart failed")
	}
	return &cart, nil
}

func (d *OrderDao) UpdateCartByUserID(ctx context.Context, userID uint64, cartID uint64, updates map[string]any) error {
	query := d.db.WithContext(ctx).Model(&model.OrderCart{}).Where("user_id = ? AND id = ?", userID, cartID)
	if err := query.Updates(updates).Error; err != nil {
		return retcode.NewError(e.MysqlERR, "update order cart failed")
	}
	if query.RowsAffected == 0 {
		return retcode.NewError(e.ErrorOrderCartNotFound, "order cart not found")
	}
	log.Printf("[DB][order] update cart rows=%d userId=%d cartId=%d updates=%v", query.RowsAffected, userID, cartID, updates)
	return nil
}

func (d *OrderDao) DeleteCartItemByID(ctx context.Context, userID uint64, cartID uint64) error {
	tx := d.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, cartID).Delete(&model.OrderCart{})
	if tx.Error != nil {
		return retcode.NewError(e.MysqlERR, "delete order cart item failed")
	}
	if tx.RowsAffected == 0 {
		return retcode.NewError(e.ErrorOrderCartNotFound, "order cart not found")
	}
	log.Printf("[DB][order] delete cart item rows=%d userId=%d cartId=%d", tx.RowsAffected, userID, cartID)
	return nil
}

func (d *OrderDao) DeleteCartByUserID(ctx context.Context, userID uint64) error {
	tx := d.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.OrderCart{})
	if tx.Error != nil {
		return retcode.NewError(e.MysqlERR, "delete order cart failed")
	}
	log.Printf("[DB][order] delete cart rows=%d userId=%d", tx.RowsAffected, userID)
	return nil
}
