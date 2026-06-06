package controller

import (
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

func (oc *OrderController) InitApiRouter(parent *gin.RouterGroup) {
	privateRouter := parent.Group("")
	privateRouter.GET("/list", oc.List)
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
