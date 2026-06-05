package admin

import (
	"sky-takeout/microservices/adminService/global"
	"sky-takeout/microservices/adminService/internal/api/controller"
	"sky-takeout/microservices/adminService/internal/repository/dao"
	"sky-takeout/microservices/adminService/internal/service"

	"sky-takeout/microservices/adminService/middle"

	"github.com/gin-gonic/gin"
)

type CategoryRouter struct{}

func (cr *CategoryRouter) InitApiRouter(parent *gin.RouterGroup) {
	//publicRouter := parent.Group("category")
	privateRouter := parent.Group("category")
	// 私有路由使用jwt验证
	privateRouter.Use(middle.VerifyJWTAdmin())
	// 依赖注入
	categoryCtrl := controller.NewCategoryController(
		service.NewCategoryService(dao.NewCategoryDao(global.DB)),
	)
	{
		privateRouter.POST("", categoryCtrl.AddCategory)
		privateRouter.GET("/page", categoryCtrl.PageQuery)
		privateRouter.GET("list", categoryCtrl.List)
		privateRouter.DELETE("", categoryCtrl.DeleteById)
		privateRouter.PUT("", categoryCtrl.EditCategory)
		privateRouter.POST("status/:status", categoryCtrl.SetStatus)

	}
}
