package model

type CreateOrderInput struct {
	UserID        uint64  `json:"userId" jsonschema:"user id"`
	GoodID        uint64  `json:"goodId" jsonschema:"goods id"`
	AddressBookID uint64  `json:"addressBookId" jsonschema:"address book id"`
	Amount        float64 `json:"amount" jsonschema:"order amount"`
	Quantity      int     `json:"quantity,omitempty" jsonschema:"goods quantity"`
}

type CreateOrderOutput struct {
	OrderID      uint64  `json:"orderId" jsonschema:"created order id"`
	OrderNo      string  `json:"orderNo" jsonschema:"created order number"`
	Status       int     `json:"status" jsonschema:"order status"`
	UserID       uint64  `json:"userId" jsonschema:"user id"`
	GoodID       uint64  `json:"goodId" jsonschema:"goods id"`
	Amount       float64 `json:"amount" jsonschema:"order amount"`
	Quantity     int     `json:"quantity" jsonschema:"goods quantity"`
	CreatedAt    string  `json:"createdAt" jsonschema:"order create time"`
	Message      string  `json:"message" jsonschema:"result message"`
	IsMockResult bool    `json:"isMockResult" jsonschema:"whether this result is mocked"`
}
