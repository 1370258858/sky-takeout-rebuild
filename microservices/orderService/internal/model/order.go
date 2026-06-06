package model

import "time"

// Request defines query params for order list.
type Request struct {
	UserID uint64 `form:"userId" json:"userId"`
	Status *int   `form:"status" json:"status"`
}

type Order struct {
	ID                  uint64    `json:"id"`
	Number              string    `json:"number"`
	Status              int       `json:"status"`
	UserID              uint64    `json:"userId" gorm:"column:user_id"`
	AddressBookID       uint64    `json:"addressBookId" gorm:"column:address_book_id"`
	OrderTime           time.Time `json:"orderTime" gorm:"column:order_time"`
	CheckoutTime        time.Time `json:"checkoutTime" gorm:"column:checkout_time"`
	PayMethod           int       `json:"payMethod" gorm:"column:pay_method"`
	PayStatus           int       `json:"payStatus" gorm:"column:pay_status"`
	Amount              float64   `json:"amount"`
	Remark              string    `json:"remark"`
	Phone               string    `json:"phone"`
	Address             string    `json:"address"`
	UserName            string    `json:"userName" gorm:"column:user_name"`
	Consignee           string    `json:"consignee"`
	CancelReason        string    `json:"cancelReason" gorm:"column:cancel_reason"`
	RejectionReason     string    `json:"rejectionReason" gorm:"column:rejection_reason"`
	CancelTime          time.Time `json:"cancelTime" gorm:"column:cancel_time"`
	EstimatedDeliveryAt time.Time `json:"estimatedDeliveryTime" gorm:"column:estimated_delivery_time"`
	DeliveryStatus      int       `json:"deliveryStatus" gorm:"column:delivery_status"`
	DeliveryTime        time.Time `json:"deliveryTime" gorm:"column:delivery_time"`
	PackAmount          int       `json:"packAmount" gorm:"column:pack_amount"`
	TablewareNumber     int       `json:"tablewareNumber" gorm:"column:tableware_number"`
	TablewareStatus     int       `json:"tablewareStatus" gorm:"column:tableware_status"`
}

func (Order) TableName() string {
	return "orders"
}
