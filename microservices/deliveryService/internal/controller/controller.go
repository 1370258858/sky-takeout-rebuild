package controller

import (
	"strconv"

	"sky-takeout/microservices/deliveryService/common/retcode"
	"sky-takeout/microservices/deliveryService/internal/model"
	"sky-takeout/microservices/deliveryService/internal/service"

	"github.com/gin-gonic/gin"
)

type DeliveryController struct {
	service service.DeliveryService
}

func NewDeliveryController(service service.DeliveryService) *DeliveryController {
	return &DeliveryController{service: service}
}

func (dc *DeliveryController) InitApiRouter(parent *gin.RouterGroup) {
	privateRouter := parent.Group("")
	privateRouter.GET("/list", dc.List)
	privateRouter.GET("/detail/:orderId", dc.DetailByOrderID)
	privateRouter.POST("/create", dc.Create)
	privateRouter.POST("/status/:orderId", dc.UpdateStatusByOrderID)
	//修改物流地址信息，修改物流留言也走这
	privateRouter.POST("/address/:orderId", dc.UpdateAddressByOrderID)
	//外卖完成后评价
	privateRouter.POST("/review/:orderId", dc.Review)

}

func (dc *DeliveryController) List(ctx *gin.Context) {
	var req model.Request
	if err := ctx.ShouldBindQuery(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	deliveries, err := dc.service.List(ctx.Request.Context(), &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, deliveries)
}

func (dc *DeliveryController) DetailByOrderID(ctx *gin.Context) {
	orderID, err := strconv.ParseUint(ctx.Param("orderId"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	delivery, err := dc.service.GetByOrderID(ctx.Request.Context(), orderID)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, delivery)
}

func (dc *DeliveryController) Create(ctx *gin.Context) {
	var req model.CreateDeliveryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	delivery, err := dc.service.Create(ctx.Request.Context(), &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, delivery)
}

func (dc *DeliveryController) UpdateStatusByOrderID(ctx *gin.Context) {
	orderID, err := strconv.ParseUint(ctx.Param("orderId"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	var req model.UpdateStatusRequest
	if err = ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	delivery, err := dc.service.UpdateStatusByOrderID(ctx.Request.Context(), orderID, &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, delivery)
}

func (dc *DeliveryController) UpdateAddressByOrderID(ctx *gin.Context) {
	orderID, err := strconv.ParseUint(ctx.Param("orderId"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	var req model.UpdateAddressRequest
	if err = ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	delivery, err := dc.service.UpdateAddressByOrderID(ctx.Request.Context(), orderID, &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, delivery)
}

func (dc *DeliveryController) Review(ctx *gin.Context) {
	orderID, err := strconv.ParseUint(ctx.Param("orderId"), 10, 64)
	if err != nil {
		retcode.Fatal(ctx, err, "invalid order id")
		return
	}
	var req model.ReviewRequest
	if err = ctx.ShouldBindJSON(&req); err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	delivery, err := dc.service.Review(ctx.Request.Context(), orderID, &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, delivery)
}
