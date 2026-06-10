package controller

import (
	"context"
	"fmt"
	"strconv"

	"sky-takeout/microservices/orderService/common/retcode"
	"sky-takeout/microservices/orderService/internal/model"
	"sky-takeout/microservices/orderService/internal/service"

	"github.com/gin-gonic/gin"
)

type OrderController struct {
	service service.OrderService
}

func NewOrderController(service service.OrderService) *OrderController {
	return &OrderController{service: service}
}

// CreateForMCP exposes create order ability for MCP adapters.
func (oc *OrderController) CreateForMCP(ctx context.Context, req *model.CreateOrderRequest) (*model.Order, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request")
	}
	return oc.service.Create(ctx, req)
}

func (oc *OrderController) InitApiRouter(parent *gin.RouterGroup) {
	privateRouter := parent.Group("")
	privateRouter.GET("/list", oc.List)
	//创建购物车
	privateRouter.POST("/cart/create", oc.createCart)
	//查看购物车
	privateRouter.GET("/cart/detail/:userId", oc.CartDetail)
	//修改购物车
	privateRouter.POST("/cart/update/:userId", oc.UpdateCart)
	//删除购物车
	privateRouter.POST("/cart/delete/:userId", oc.DeleteCart)
	//创建订单  rd 生成id 订单创建一般是幂等的，所以可以通过客户端生成一个唯一id，服务端根据这个id进行幂等处理，如果订单已经存在就返回订单信息，如果订单不存在就创建订单并返回订单信息
	//创建订单 id 目前在服务端雪花id生产 后续再加rd逻辑
	privateRouter.POST("/create", oc.Create)

	//获取订单详情
	privateRouter.GET("/detail/:id", oc.Detail)
	//取消订单
	privateRouter.POST("/cancel/:id", oc.Cancel)
	//支付订单  支付成功后mq塞延时消息，延时消息到期后回调订单超时接口，如果订单已经支付成功了就不处理，如果订单还未支付成功就取消订单
	privateRouter.POST("/pay/:id", oc.Pay)
	//支付超时回调 //模拟超时 超时一般是mq回调或者第三方支付回调 mq回调ctl层
	privateRouter.POST("/pay_timeout/:id", oc.PayTimeout)
	//订单退款  支付前调用退款接口，直接取消订单；支付后调用退款接口，调用第三方支付退款接口，退款成功后取消订单
	privateRouter.POST("/refund/:id", oc.Refund)

}

func (oc *OrderController) createCart(ctx *gin.Context) {
	var req model.CreateCartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	cart, err := oc.service.CreateCart(ctx.Request.Context(), &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, cart)
}

func (oc *OrderController) CartDetail(ctx *gin.Context) {
	userID, err := strconv.ParseUint(ctx.Param("userId"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid user id")
		return
	}
	carts, err := oc.service.GetCart(ctx.Request.Context(), userID)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, carts)
}

func (oc *OrderController) UpdateCart(ctx *gin.Context) {
	userID, err := strconv.ParseUint(ctx.Param("userId"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid user id")
		return
	}
	var req model.UpdateCartRequest
	if err = ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	cart, err := oc.service.UpdateCart(ctx.Request.Context(), userID, &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, cart)
}

func (oc *OrderController) DeleteCart(ctx *gin.Context) {
	userID, err := strconv.ParseUint(ctx.Param("userId"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid user id")
		return
	}
	var req model.DeleteCartRequest
	_ = ctx.ShouldBindJSON(&req)
	if err = oc.service.DeleteCart(ctx.Request.Context(), userID, &req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, gin.H{"userId": userID, "deleted": true, "cartId": req.CartID})
}

func (oc *OrderController) Create(ctx *gin.Context) {
	var req model.CreateOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	order, err := oc.service.Create(ctx.Request.Context(), &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, order)
}

func (oc *OrderController) Detail(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	order, err := oc.service.Detail(ctx.Request.Context(), id)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, order)
}

func (oc *OrderController) Cancel(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	var req model.CancelOrderRequest
	_ = ctx.ShouldBindJSON(&req)
	order, err := oc.service.Cancel(ctx.Request.Context(), id, &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, order)
}

func (oc *OrderController) Pay(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	var req model.PayOrderRequest
	if err = ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	order, err := oc.service.Pay(ctx.Request.Context(), id, &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, order)
}

func (oc *OrderController) PayTimeout(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	order, err := oc.service.PayTimeout(ctx.Request.Context(), id)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, order)
}

func (oc *OrderController) Refund(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	var req model.RefundOrderRequest
	_ = ctx.ShouldBindJSON(&req)
	order, err := oc.service.Refund(ctx.Request.Context(), id, &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, order)
}

func (oc *OrderController) List(ctx *gin.Context) {
	var req model.Request
	if err := ctx.ShouldBindQuery(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	orders, err := oc.service.List(ctx.Request.Context(), &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, orders)
}

//mcp func write here
