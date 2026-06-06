package controller

import (
	"sky-takeout/microservices/goodsService/common/retcode"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/model"
	"sky-takeout/microservices/goodsService/internal/service"

	"github.com/gin-gonic/gin"
)

type DishController struct {
	service service.DishService
}

func NewDishController(service service.DishService) *DishController {
	return &DishController{service: service}
}

func (cr *DishController) InitApiRouter(parent *gin.RouterGroup) {
	privateRouter := parent.Group("/category")
	privateRouter.GET("/list", cr.List)
}

// 这里的controller层主要负责接收请求，参数校验，调用service层处理业务逻辑，最后返回结果给客户端
func (cc *DishController) List(ctx *gin.Context) {
	var req model.Resquest
	err := ctx.Bind(&req)
	if err != nil {
		global.Log.DebugContext(ctx, "param CategoryDTO json failed err=%s", err.Error())
		retcode.Fatal(ctx, err, "")
		return
	}
	dish, err := cc.service.List(ctx.Request.Context(), &req)
	if err != nil {
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, dish)
}
