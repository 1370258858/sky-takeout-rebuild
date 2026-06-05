package admin

import (
	"sky-takeout/microservices/adminService/global"
	"sky-takeout/microservices/adminService/internal/api/controller"
	"sky-takeout/microservices/adminService/internal/repository/dao"
	"sky-takeout/microservices/adminService/internal/service"

	"sky-takeout/microservices/adminService/middle"

	"github.com/gin-gonic/gin"
)

type SetMealRouter struct{}

func (sr *SetMealRouter) InitApiRouter(parent *gin.RouterGroup) {
	//publicRouter := parent.Group("category")
	privateRouter := parent.Group("setmeal")
	// 私有路由使用jwt验证
	privateRouter.Use(middle.VerifyJWTAdmin())
	// 依赖注入
	setmealCtrl := controller.NewSetMealController(
		service.NewSetMealService(dao.NewSetMealDao(global.DB), dao.NewSetMealDishDao()),
	)
	{
		privateRouter.POST("", setmealCtrl.SaveWithDish)
		privateRouter.GET("/page", setmealCtrl.PageQuery)
		privateRouter.GET("/:id", setmealCtrl.GetByIdWithDish)
		privateRouter.POST("/status/:status", setmealCtrl.OnOrClose)
		privateRouter.DELETE("", setmealCtrl.Delete)
	}
}
