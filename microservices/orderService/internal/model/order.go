package model

import "time"

// Request defines query params for order list.
type Request struct {
	UserID uint64 `form:"userId" json:"userId"`
	Status int    `form:"status" json:"status"`
}

// CreateOrderRequest defines the request for creating an order.
// Refer to CreateForMCP() in controller for detailed usage documentation.
type CreateOrderRequest struct {
	GoodIDs               []int64 `json:"goodIds"`
	Quantity              int     `json:"quantity"`
	UserID                uint64  `json:"userId" binding:"required"`
	AddressBookID         uint64  `json:"addressBookId" binding:"required"`
	PayMethod             int     `json:"payMethod"`
	Amount                float64 `json:"amount"`
	Remark                string  `json:"remark"`
	CartID                uint64  `json:"cartId" `
	Phone                 string  `json:"phone"`
	Address               string  `json:"address"`
	UserName              string  `json:"userName"`
	Consignee             string  `json:"consignee"`
	EstimatedDeliveryTime string  `json:"estimatedDeliveryTime"`
	PackAmount            int     `json:"packAmount"`
	TablewareNumber       int     `json:"tablewareNumber"`
	TablewareStatus       int     `json:"tablewareStatus"`
}

type PayOrderRequest struct {
	PayStatus int `json:"payStatus" binding:"required"`
}

type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

type RefundOrderRequest struct {
	Reason  string `json:"reason"`
	OrderID uint64 `json:"orderId"`
}

type OrderTimeoutMessage struct {
	OrderID uint64 `json:"orderId"`
}

type Order struct {
	ID                  uint64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Number              string     `json:"number" gorm:"column:number"`
	GoodIDs             []int64    `json:"goodIds" gorm:"column:good_ids;type:json;serializer:json"`
	Status              int        `json:"status" gorm:"column:status"`
	UserID              uint64     `json:"userId" gorm:"column:user_id"`
	AddressBookID       uint64     `json:"addressBookId" gorm:"column:address_book_id"`
	OrderTime           *time.Time `json:"orderTime" gorm:"column:order_time"`
	CheckoutTime        *time.Time `json:"checkoutTime,omitempty" gorm:"column:checkout_time"`
	PayMethod           int        `json:"payMethod" gorm:"column:pay_method"`
	PayStatus           int        `json:"payStatus" gorm:"column:pay_status"`
	Amount              float64    `json:"amount" gorm:"column:amount"`
	Remark              string     `json:"remark" gorm:"column:remark"`
	Phone               string     `json:"phone" gorm:"column:phone"`
	Address             string     `json:"address" gorm:"column:address"`
	UserName            string     `json:"userName" gorm:"column:user_name"`
	Consignee           string     `json:"consignee" gorm:"column:consignee"`
	CancelReason        string     `json:"cancelReason" gorm:"column:cancel_reason"`
	RejectionReason     string     `json:"rejectionReason" gorm:"column:rejection_reason"`
	CancelTime          *time.Time `json:"cancelTime,omitempty" gorm:"column:cancel_time"`
	EstimatedDeliveryAt *time.Time `json:"estimatedDeliveryTime,omitempty" gorm:"column:estimated_delivery_time"`
	DeliveryStatus      int        `json:"deliveryStatus" gorm:"column:delivery_status"`
	DeliveryTime        *time.Time `json:"deliveryTime,omitempty" gorm:"column:delivery_time"`
	PackAmount          int        `json:"packAmount" gorm:"column:pack_amount"`
	TablewareNumber     int        `json:"tablewareNumber" gorm:"column:tableware_number"`
	TablewareStatus     int        `json:"tablewareStatus" gorm:"column:tableware_status"`
}

type CreateCartRequest struct {
	UserID   uint64  `json:"userId" binding:"required"`
	GoodIDs  []int64 `json:"goodIds" binding:"required"`
	Quantity int     `json:"quantity" binding:"required"`
	Name     string  `json:"name"`
	Image    string  `json:"image"`
	Flavor   string  `json:"flavor"`
	Amount   float64 `json:"amount"`
}

type UpdateCartRequest struct {
	UserID   uint64  `json:"userId" binding:"required"`
	CartID   uint64  `json:"cartId" binding:"required"`
	Quantity int     `json:"quantity" binding:"required"`
	Flavor   string  `json:"flavor"`
	Amount   float64 `json:"amount"`
}

type DeleteCartRequest struct {
	UserID uint64 `json:"userId"`
	CartID uint64 `json:"cartId"`
}

// UpdateFact defines a single fact update item returned by MCP tools.
type UpdateFact struct {
	Key        string `json:"key"`
	Value      any    `json:"value"`
	Confidence string `json:"confidence,omitempty"`
	Source     string `json:"source,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// OrderWithUpdateFacts flattens Order fields and updateFacts at the same level.
type OrderWithUpdateFacts struct {
	*Order
	UpdateFacts []UpdateFact `json:"updateFacts,omitempty"`
}

// OrderCartWithUpdateFacts flattens OrderCart fields and updateFacts at the same level.
type OrderCartWithUpdateFacts struct {
	*OrderCart
	UpdateFacts []UpdateFact `json:"updateFacts,omitempty"`
}

type OrderCart struct {
	ID         uint64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Name       string     `json:"name" gorm:"column:name"`
	Image      string     `json:"image" gorm:"column:image"`
	UserID     uint64     `json:"userId" gorm:"column:user_id"`
	GoodIDs    []int64    `json:"goodIds" gorm:"column:good_ids;type:json;serializer:json"`
	SetMealID  *uint64    `json:"setmealId,omitempty" gorm:"column:setmeal_id"`
	Flavor     string     `json:"flavor" gorm:"column:dish_flavor"`
	Quantity   int        `json:"quantity" gorm:"column:number"`
	Amount     float64    `json:"amount" gorm:"column:amount"`
	CreateTime *time.Time `json:"createTime,omitempty" gorm:"column:create_time"`
	UpdateTime *time.Time `json:"updateTime,omitempty" gorm:"column:update_time"`
}

type OrderDetail struct {
	ID         uint64  `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Name       string  `json:"name" gorm:"column:name"`
	Image      string  `json:"image" gorm:"column:image"`
	OrderID    uint64  `json:"orderId" gorm:"column:order_id"`
	DishID     *uint64 `json:"dishId,omitempty" gorm:"column:dish_id"`
	SetMealID  *uint64 `json:"setmealId,omitempty" gorm:"column:setmeal_id"`
	DishFlavor string  `json:"dishFlavor" gorm:"column:dish_flavor"`
	Number     int     `json:"number" gorm:"column:number"`
	Amount     float64 `json:"amount" gorm:"column:amount"`
}

type CartDetailRequest struct {
	UserID uint64 `json:"userId" binding:"required"`
}

func (OrderCart) TableName() string {
	return "shopping_cart"
}

func (Order) TableName() string {
	return "orders"
}

func (OrderDetail) TableName() string {
	return "order_detail"
}
