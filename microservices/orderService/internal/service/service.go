package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sky-takeout/microservices/orderService/internal/model"
	"sky-takeout/microservices/orderService/internal/repository/dao"
	"strconv"
	"time"

	"sky-takeout/microservices/orderService/common/e"
	"sky-takeout/microservices/orderService/common/retcode"
	"sky-takeout/microservices/orderService/global"
	deliveryv1 "sky-takeout/microservices/rpc/pb/deliveryv1"
	goodsv1 "sky-takeout/microservices/rpc/pb/goodsv1"
)

const (
	orderStatusPendingPay    = 1
	orderStatusPendingAccept = 2
	orderStatusCanceled      = 6
	orderStatusRefunded      = 7

	payStatusUnpaid   = 0
	payStatusPaid     = 1
	payStatusRefunded = 2

	payRequestSuccess = 1
	payRequestFailed  = 2
	payRequestTimeout = 3
)

type TimeoutPublisher func(ctx context.Context, orderID uint64) error

type OrderService interface {
	List(ctx context.Context, req *model.Request) ([]model.Order, error)
	Create(ctx context.Context, req *model.CreateOrderRequest) (*model.Order, error)
	CreateCart(ctx context.Context, req *model.CreateCartRequest) (*model.OrderCart, error)
	GetCart(ctx context.Context, userID uint64) ([]model.OrderCart, error)
	UpdateCart(ctx context.Context, userID uint64, req *model.UpdateCartRequest) (*model.OrderCart, error)
	DeleteCart(ctx context.Context, userID uint64, req *model.DeleteCartRequest) error
	Detail(ctx context.Context, id uint64) (*model.Order, error)
	Cancel(ctx context.Context, id uint64, req *model.CancelOrderRequest) (*model.Order, error)
	Pay(ctx context.Context, id uint64, req *model.PayOrderRequest) (*model.Order, error)
	PayTimeout(ctx context.Context, id uint64) (*model.Order, error)
	Refund(ctx context.Context, id uint64, req *model.RefundOrderRequest) (*model.Order, error)
}

type OrderServiceImpl struct {
	repo             *dao.OrderDao
	timeoutPublisher TimeoutPublisher
}

func NewOrderService(repo *dao.OrderDao, timeoutPublisher TimeoutPublisher) OrderService {
	return &OrderServiceImpl{repo: repo, timeoutPublisher: timeoutPublisher}
}

func (s *OrderServiceImpl) List(ctx context.Context, req *model.Request) ([]model.Order, error) {
	log.Printf("[SVC][order] list request userId=%d status=%v", req.UserID, req.Status)
	orders, err := s.repo.List(ctx, *req)
	if err != nil {
		log.Printf("[SVC][order][ERR] list failed userId=%d status=%v err=%v", req.UserID, req.Status, err)
		return nil, err
	}
	return orders, nil
}

func (s *OrderServiceImpl) Create(ctx context.Context, req *model.CreateOrderRequest) (*model.Order, error) {
	if global.GoodsRPCClient == nil {
		log.Printf("[SVC][order][ERR] create failed goods rpc client not initialized userId=%d goodId=%d", req.UserID, req.GoodID)
		return nil, retcode.NewError(e.ERROR, "goods rpc client not initialized")
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	log.Printf("[SVC][order] create request userId=%d goodId=%d quantity=%d amount=%.2f", req.UserID, req.GoodID, req.Quantity, req.Amount)

	log.Printf("[RPC][order->goods] method=GetGoodById req={id:%d}", req.GoodID)
	good, err := global.GoodsRPCClient.GetGoodById(ctx, &goodsv1.GetGoodByIdRequest{Id: req.GoodID})
	if err != nil {
		log.Printf("[SVC][order][ERR] create failed goods rpc error goodId=%d err=%v", req.GoodID, err)
		return nil, retcode.NewError(e.ERROR, "query goods rpc failed")
	}
	log.Printf("[RPC][order->goods] method=GetGoodById resp={id:%d,name:%s,status:%d}", good.GetId(), good.GetName(), good.GetStatus())
	if good.GetId() == 0 {
		log.Printf("[SVC][order][ERR] create failed goods not found goodId=%d", req.GoodID)
		return nil, retcode.NewError(e.ErrorOrderNotFound, "goods not found")
	}

	orderID := NextOrderID()
	now := time.Now()
	order := &model.Order{
		ID:              orderID,
		Number:          strconv.FormatUint(orderID, 10),
		Status:          orderStatusPendingPay,
		UserID:          req.UserID,
		AddressBookID:   req.AddressBookID,
		OrderTime:       &now,
		PayMethod:       req.PayMethod,
		PayStatus:       payStatusUnpaid,
		Amount:          req.Amount,
		Remark:          req.Remark,
		Phone:           req.Phone,
		Address:         req.Address,
		UserName:        req.UserName,
		Consignee:       req.Consignee,
		PackAmount:      req.PackAmount,
		TablewareNumber: req.TablewareNumber,
		TablewareStatus: req.TablewareStatus,
	}
	if req.PayMethod == 0 {
		order.PayMethod = 1
	}
	if req.EstimatedDeliveryTime != "" {
		if t, parseErr := time.ParseInLocation("2006-01-02 15:04:05", req.EstimatedDeliveryTime, time.Local); parseErr == nil {
			order.EstimatedDeliveryAt = &t
		}
	}

	created, err := s.repo.Create(ctx, order)
	if err != nil {
		log.Printf("[SVC][order][ERR] create failed persist orderId=%d userId=%d err=%v", orderID, req.UserID, err)
		return nil, err
	}
	log.Printf("[SVC][order] create success orderId=%d number=%s", created.ID, created.Number)

	// Current implementation generates order id inside pod via snowflake node=1.
	// Keep this point as extension hook for future RD-generated id access.
	if s.timeoutPublisher != nil {
		log.Printf("[MQ][order] publish timeout message orderId=%d delayMs=%d", created.ID, 3*60*1000)
		if err = s.timeoutPublisher(ctx, created.ID); err != nil {
			log.Printf("[SVC][order][ERR] create failed publish timeout message orderId=%d err=%v", created.ID, err)
			return nil, err
		}
	}
	return created, nil
}

func (s *OrderServiceImpl) Detail(ctx context.Context, id uint64) (*model.Order, error) {
	log.Printf("[SVC][order] detail request orderId=%d", id)
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] detail failed orderId=%d err=%v", id, err)
		return nil, err
	}
	return order, nil
}

func (s *OrderServiceImpl) Cancel(ctx context.Context, id uint64, req *model.CancelOrderRequest) (*model.Order, error) {
	reason := "manual cancel"
	if req != nil && req.Reason != "" {
		reason = req.Reason
	}
	err := s.repo.UpdateByID(ctx, id, map[string]any{
		"status":        orderStatusCanceled,
		"cancel_reason": reason,
		"cancel_time":   time.Now(),
	})
	if err != nil {
		log.Printf("[SVC][order][ERR] cancel failed orderId=%d reason=%s err=%v", id, reason, err)
		return nil, err
	}
	log.Printf("[SVC][order] cancel success orderId=%d reason=%s", id, reason)
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] cancel failed when reloading orderId=%d err=%v", id, err)
		return nil, err
	}
	return order, nil
}

func (s *OrderServiceImpl) Pay(ctx context.Context, id uint64, req *model.PayOrderRequest) (*model.Order, error) {
	if req == nil {
		log.Printf("[SVC][order][ERR] pay failed invalid request orderId=%d", id)
		return nil, retcode.NewError(e.ERROR, "invalid pay request")
	}
	log.Printf("[SVC][order] pay request orderId=%d payStatus=%d", id, req.PayStatus)
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] pay failed query order orderId=%d err=%v", id, err)
		return nil, err
	}

	switch req.PayStatus {
	case payRequestSuccess:
		err = s.repo.UpdateByID(ctx, id, map[string]any{
			"pay_status":    payStatusPaid,
			"status":        orderStatusPendingAccept,
			"checkout_time": time.Now(),
		})
		if err != nil {
			log.Printf("[SVC][order][ERR] pay failed update paid status orderId=%d err=%v", id, err)
			return nil, err
		}
		if global.DeliveryRPCClient != nil {
			goodsSnapshot := map[string]any{"orderId": id, "amount": order.Amount}
			payload, _ := json.Marshal(goodsSnapshot)
			log.Printf("[RPC][order->delivery] method=CreateDelivery req={orderId:%d,deliveryNo:%s,status:%d,goodsInfo:%s}", id, fmt.Sprintf("DL%s", order.Number), 1, string(payload))
			if _, err = global.DeliveryRPCClient.CreateDelivery(ctx, &deliveryv1.CreateDeliveryRequest{
				OrderId:         id,
				DeliveryNo:      fmt.Sprintf("DL%s", order.Number),
				Status:          1,
				GoodsInfo:       string(payload),
				PickupAddress:   "",
				DeliveryAddress: order.Address,
				Remark:          "呼叫骑手中",
			}); err != nil {
				log.Printf("[SVC][order][ERR] pay failed create delivery orderId=%d err=%v", id, err)
				return nil, retcode.NewError(e.ERROR, "create delivery rpc failed")
			}
			log.Printf("[RPC][order->delivery] method=CreateDelivery resp=ok orderId=%d", id)
		}
	case payRequestFailed:
		return order, nil
	case payRequestTimeout:
		return s.PayTimeout(ctx, id)
	default:
		log.Printf("[SVC][order][ERR] pay failed unsupported status orderId=%d payStatus=%d", id, req.PayStatus)
		return nil, retcode.NewError(e.ERROR, "unsupported pay status")
	}

	updatedOrder, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] pay failed when reloading orderId=%d err=%v", id, err)
		return nil, err
	}
	return updatedOrder, nil
}

func (s *OrderServiceImpl) PayTimeout(ctx context.Context, id uint64) (*model.Order, error) {
	log.Printf("[SVC][order] pay-timeout handling orderId=%d", id)
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] pay-timeout failed query orderId=%d err=%v", id, err)
		return nil, err
	}
	if order.PayStatus == payStatusPaid {
		return order, nil
	}
	err = s.repo.UpdateByID(ctx, id, map[string]any{
		"status":        orderStatusCanceled,
		"cancel_reason": "pay timeout",
		"cancel_time":   time.Now(),
	})
	if err != nil {
		log.Printf("[SVC][order][ERR] pay-timeout failed update canceled orderId=%d err=%v", id, err)
		return nil, err
	}
	log.Printf("[SVC][order] pay-timeout canceled orderId=%d", id)
	updatedOrder, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] pay-timeout failed when reloading orderId=%d err=%v", id, err)
		return nil, err
	}
	return updatedOrder, nil
}

func (s *OrderServiceImpl) Refund(ctx context.Context, id uint64, req *model.RefundOrderRequest) (*model.Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] refund failed query orderId=%d err=%v", id, err)
		return nil, err
	}
	if order.PayStatus != payStatusPaid {
		log.Printf("[SVC][order][ERR] refund rejected orderId=%d payStatus=%d", id, order.PayStatus)
		return nil, retcode.NewError(e.ErrorOrderStatusError, "order is not paid")
	}
	log.Printf("[SVC][order] refund request orderId=%d", id)

	if global.DeliveryRPCClient != nil {
		log.Printf("[RPC][order->delivery] method=GetDeliveryByOrderId req={orderId:%d}", id)
		if _, err = global.DeliveryRPCClient.GetDeliveryByOrderId(ctx, &deliveryv1.GetDeliveryByOrderIdRequest{OrderId: id}); err != nil {
			log.Printf("[SVC][order][ERR] refund failed query delivery orderId=%d err=%v", id, err)
			return nil, retcode.NewError(e.ERROR, "query delivery status rpc failed")
		}
		log.Printf("[RPC][order->delivery] method=GetDeliveryByOrderId resp=ok orderId=%d", id)
		reason := "取消物流成功"
		if req != nil && req.Reason != "" {
			reason = req.Reason
		}
		log.Printf("[RPC][order->delivery] method=UpdateDeliveryByOrderId req={orderId:%d,status:%d,remark:%s}", id, 6, reason)
		if _, err = global.DeliveryRPCClient.UpdateDeliveryByOrderId(ctx, &deliveryv1.UpdateDeliveryByOrderIdRequest{
			OrderId: id,
			Status:  6,
			Remark:  reason,
		}); err != nil {
			log.Printf("[SVC][order][ERR] refund failed update delivery orderId=%d err=%v", id, err)
			return nil, retcode.NewError(e.ERROR, "update delivery by order id rpc failed")
		}
		log.Printf("[RPC][order->delivery] method=UpdateDeliveryByOrderId resp=ok orderId=%d", id)
	}

	err = s.repo.UpdateByID(ctx, id, map[string]any{
		"pay_status": payStatusRefunded,
		"status":     orderStatusRefunded,
	})
	if err != nil {
		log.Printf("[SVC][order][ERR] refund failed update orderId=%d err=%v", id, err)
		return nil, err
	}
	log.Printf("[SVC][order] refund success orderId=%d", id)
	updatedOrder, err := s.repo.GetByID(ctx, id)
	if err != nil {
		log.Printf("[SVC][order][ERR] refund failed when reloading orderId=%d err=%v", id, err)
		return nil, err
	}
	return updatedOrder, nil
}

func (s *OrderServiceImpl) CreateCart(ctx context.Context, req *model.CreateCartRequest) (*model.OrderCart, error) {
	if req == nil {
		log.Printf("[SVC][order][ERR] create cart failed nil request")
		return nil, retcode.NewError(e.ERROR, "invalid create cart request")
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}
	log.Printf("[SVC][order] create cart request userId=%d goodId=%d quantity=%d", req.UserID, req.GoodID, req.Quantity)

	createdCart, err := s.repo.CreateOrderCart(ctx, &model.OrderCart{
		Name:     req.Name,
		Image:    req.Image,
		UserID:   req.UserID,
		GoodID:   req.GoodID,
		Flavor:   req.Flavor,
		Quantity: req.Quantity,
		Amount:   req.Amount,
	})
	if err != nil {
		log.Printf("[SVC][order][ERR] create cart failed persist userId=%d goodId=%d err=%v", req.UserID, req.GoodID, err)
		return nil, err
	}
	log.Printf("[SVC][order] create cart success orderCartId=%d userId=%d goodId=%d", createdCart.ID, req.UserID, req.GoodID)
	return createdCart, nil
}

func (s *OrderServiceImpl) GetCart(ctx context.Context, userID uint64) ([]model.OrderCart, error) {
	log.Printf("[SVC][order] cart detail request userId=%d", userID)
	items, err := s.repo.GetCartByUserID(ctx, userID)
	if err != nil {
		log.Printf("[SVC][order][ERR] cart detail failed userId=%d err=%v", userID, err)
		return nil, err
	}
	return items, nil
}

func (s *OrderServiceImpl) UpdateCart(ctx context.Context, userID uint64, req *model.UpdateCartRequest) (*model.OrderCart, error) {
	if req == nil {
		log.Printf("[SVC][order][ERR] update cart failed nil request userId=%d", userID)
		return nil, retcode.NewError(e.ERROR, "invalid update cart request")
	}
	if req.Quantity <= 0 {
		return nil, retcode.NewError(e.ERROR, "quantity must be greater than 0")
	}
	updates := map[string]any{
		"number": req.Quantity,
	}
	if req.Flavor != "" {
		updates["dish_flavor"] = req.Flavor
	}
	if req.Amount > 0 {
		updates["amount"] = req.Amount
	}
	if err := s.repo.UpdateCartByUserID(ctx, userID, req.CartID, updates); err != nil {
		log.Printf("[SVC][order][ERR] update cart failed userId=%d cartId=%d err=%v", userID, req.CartID, err)
		return nil, err
	}
	item, err := s.repo.GetCartItemByID(ctx, userID, req.CartID)
	if err != nil {
		log.Printf("[SVC][order][ERR] update cart reload failed userId=%d cartId=%d err=%v", userID, req.CartID, err)
		return nil, err
	}
	return item, nil
}

func (s *OrderServiceImpl) DeleteCart(ctx context.Context, userID uint64, req *model.DeleteCartRequest) error {
	if req != nil && req.CartID > 0 {
		if err := s.repo.DeleteCartItemByID(ctx, userID, req.CartID); err != nil {
			log.Printf("[SVC][order][ERR] delete cart item failed userId=%d cartId=%d err=%v", userID, req.CartID, err)
			return err
		}
		return nil
	}
	if err := s.repo.DeleteCartByUserID(ctx, userID); err != nil {
		log.Printf("[SVC][order][ERR] delete cart failed userId=%d err=%v", userID, err)
		return err
	}
	return nil
}
