package dao

import (
	"context"
	"log"
	"sky-takeout/microservices/deliveryService/common/e"
	"sky-takeout/microservices/deliveryService/common/retcode"
	"sky-takeout/microservices/deliveryService/internal/model"
	"time"

	"gorm.io/gorm"
)

type DeliveryDao struct {
	db *gorm.DB
}

func NewDeliveryDao(db *gorm.DB) *DeliveryDao {
	return &DeliveryDao{db: db}
}

func (d *DeliveryDao) List(ctx context.Context, req model.Request) ([]model.Delivery, error) {
	var deliveries []model.Delivery
	query := d.db.WithContext(ctx).Model(&model.Delivery{})
	if req.OrderID > 0 {
		query = query.Where("order_id = ?", req.OrderID)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	tx := query.Order("id desc").Find(&deliveries)
	if tx.Error != nil {
		return nil, retcode.NewError(e.MysqlERR, "list delivery failed")
	}
	log.Printf("[DB][delivery] query list rows=%d orderId=%d status=%v", tx.RowsAffected, req.OrderID, req.Status)
	return deliveries, nil
}

func (d *DeliveryDao) GetByOrderID(ctx context.Context, orderID uint64) (*model.Delivery, error) {
	var delivery model.Delivery
	tx := d.db.WithContext(ctx).Model(&model.Delivery{}).Where("order_id = ?", orderID).Order("id desc").Take(&delivery)
	if tx.Error != nil {
		if tx.Error == gorm.ErrRecordNotFound {
			return nil, retcode.NewError(e.ErrorOrderNotFound, "delivery not found")
		}
		return nil, retcode.NewError(e.MysqlERR, "query delivery failed")
	}
	log.Printf("[DB][delivery] query by orderId rows=%d orderId=%d", tx.RowsAffected, orderID)
	return &delivery, nil
}

func (d *DeliveryDao) Create(ctx context.Context, in *model.Delivery) (*model.Delivery, error) {
	tx := d.db.WithContext(ctx).Create(in)
	if tx.Error != nil {
		return nil, retcode.NewError(e.MysqlERR, "create delivery failed")
	}
	log.Printf("[DB][delivery] insert rows=%d orderId=%d deliveryNo=%s", tx.RowsAffected, in.OrderID, in.DeliveryNo)
	return in, nil
}

func (d *DeliveryDao) UpdateStatusByOrderID(ctx context.Context, orderID uint64, req model.UpdateStatusRequest) (*model.Delivery, error) {
	updates := map[string]interface{}{
		"status":      req.Status,
		"remark":      req.Remark,
		"update_time": time.Now(),
	}
	if req.RiderID != nil {
		updates["rider_id"] = *req.RiderID
	}
	if req.RiderName != "" {
		updates["rider_name"] = req.RiderName
	}
	if req.RiderPhone != "" {
		updates["rider_phone"] = req.RiderPhone
	}
	now := time.Now()
	switch req.Status {
	case 2:
		updates["dispatch_time"] = now
	case 3:
		updates["pickup_time"] = now
	case 4:
		updates["delivered_time"] = now
	}

	query := d.db.WithContext(ctx).Model(&model.Delivery{}).Where("order_id = ?", orderID)
	if err := query.Updates(updates).Error; err != nil {
		return nil, retcode.NewError(e.MysqlERR, "update delivery status failed")
	}
	if query.RowsAffected == 0 {
		return nil, retcode.NewError(e.ErrorOrderNotFound, "delivery not found")
	}
	log.Printf("[DB][delivery] update rows=%d orderId=%d updates=%v", query.RowsAffected, orderID, updates)
	return d.GetByOrderID(ctx, orderID)
}

func (d *DeliveryDao) UpdateByOrderIDInsert(ctx context.Context, orderID uint64, req model.UpdateStatusRequest) (*model.Delivery, error) {
	current, err := d.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	entity := &model.Delivery{
		OrderID:             current.OrderID,
		DeliveryNo:          current.DeliveryNo,
		Status:              req.Status,
		GoodsInfo:           current.GoodsInfo,
		RiderID:             current.RiderID,
		RiderName:           current.RiderName,
		RiderPhone:          current.RiderPhone,
		PickupAddress:       current.PickupAddress,
		DeliveryAddress:     current.DeliveryAddress,
		DispatchTime:        current.DispatchTime,
		PickupTime:          current.PickupTime,
		ExpectedArrivalTime: current.ExpectedArrivalTime,
		DeliveredTime:       current.DeliveredTime,
		Remark:              req.Remark,
		CreateTime:          &now,
		UpdateTime:          &now,
	}
	if req.RiderID != nil {
		entity.RiderID = req.RiderID
	}
	if req.RiderName != "" {
		entity.RiderName = req.RiderName
	}
	if req.RiderPhone != "" {
		entity.RiderPhone = req.RiderPhone
	}
	tx := d.db.WithContext(ctx).Create(entity)
	if tx.Error != nil {
		return nil, retcode.NewError(e.MysqlERR, "insert delivery history failed")
	}
	log.Printf("[DB][delivery] insert history rows=%d orderId=%d status=%d", tx.RowsAffected, orderID, req.Status)
	return entity, nil
}
