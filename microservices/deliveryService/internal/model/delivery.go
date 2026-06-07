package model

import "time"

type Request struct {
	OrderID uint64 `form:"orderId" json:"orderId"`
	Status  *int   `form:"status" json:"status"`
}

type CreateDeliveryRequest struct {
	OrderID             uint64  `json:"orderId" binding:"required"`
	DeliveryNo          string  `json:"deliveryNo"`
	Status              int     `json:"status"`
	GoodsInfo           string  `json:"goodsInfo" binding:"required"`
	RiderID             *uint64 `json:"riderId"`
	RiderName           string  `json:"riderName"`
	RiderPhone          string  `json:"riderPhone"`
	PickupAddress       string  `json:"pickupAddress"`
	DeliveryAddress     string  `json:"deliveryAddress"`
	ExpectedArrivalTime string  `json:"expectedArrivalTime"`
	Remark              string  `json:"remark"`
}

type UpdateStatusRequest struct {
	Status     int     `json:"status" binding:"required"`
	Remark     string  `json:"remark"`
	RiderID    *uint64 `json:"riderId"`
	RiderName  string  `json:"riderName"`
	RiderPhone string  `json:"riderPhone"`
}

type Delivery struct {
	ID                  uint64     `json:"id" gorm:"column:id"`
	OrderID             uint64     `json:"orderId" gorm:"column:order_id"`
	DeliveryNo          string     `json:"deliveryNo" gorm:"column:delivery_no"`
	Status              int        `json:"status" gorm:"column:status"`
	GoodsInfo           string     `json:"goodsInfo" gorm:"column:goods_info"`
	RiderID             *uint64    `json:"riderId" gorm:"column:rider_id"`
	RiderName           string     `json:"riderName" gorm:"column:rider_name"`
	RiderPhone          string     `json:"riderPhone" gorm:"column:rider_phone"`
	PickupAddress       string     `json:"pickupAddress" gorm:"column:pickup_address"`
	DeliveryAddress     string     `json:"deliveryAddress" gorm:"column:delivery_address"`
	DispatchTime        *time.Time `json:"dispatchTime" gorm:"column:dispatch_time"`
	PickupTime          *time.Time `json:"pickupTime" gorm:"column:pickup_time"`
	ExpectedArrivalTime *time.Time `json:"expectedArrivalTime" gorm:"column:expected_arrival_time"`
	DeliveredTime       *time.Time `json:"deliveredTime" gorm:"column:delivered_time"`
	Remark              string     `json:"remark" gorm:"column:remark"`
	CreateTime          *time.Time `json:"createTime" gorm:"column:create_time"`
	UpdateTime          *time.Time `json:"updateTime" gorm:"column:update_time"`
}

func (Delivery) TableName() string {
	return "delivery"
}
