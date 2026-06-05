package admin

import (
	"sky-takeout/microservices/adminService/global"
	"sky-takeout/microservices/adminService/internal/api/controller"
	"sky-takeout/microservices/adminService/internal/repository/dao"
	"sky-takeout/microservices/adminService/internal/service"

	"sky-takeout/microservices/adminService/middle"

	"github.com/gin-gonic/gin"
)

type DishRouter struct{}

func (dr *DishRouter) InitApiRouter(parent *gin.RouterGroup) {
	//publicRouter := parent.Group("category")
	privateRouter := parent.Group("dish")
	// 私有路由使用jwt验证
	privateRouter.Use(middle.VerifyJWTAdmin())
	// 依赖注入
	dishCtrl := controller.NewDishController(
		service.NewDishService(
			dao.NewDishRepo(global.DB),
			dao.NewDishFlavorDao(global.DB),
		),
	)
	{
		privateRouter.POST("", dishCtrl.AddDish)
		privateRouter.GET("/page", dishCtrl.PageQuery)
		privateRouter.GET("/:id", dishCtrl.GetById)
		privateRouter.GET("/list", dishCtrl.List)
		privateRouter.POST("/status/:status", dishCtrl.OnOrClose)
		privateRouter.PUT("", dishCtrl.Update)
		privateRouter.DELETE("", dishCtrl.Delete)
	}
}
