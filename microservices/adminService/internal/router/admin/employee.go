package admin

import (
	"sky-takeout/microservices/adminService/global"
	"sky-takeout/microservices/adminService/internal/api/controller"
	"sky-takeout/microservices/adminService/internal/repository/dao"
	"sky-takeout/microservices/adminService/internal/service"

	"sky-takeout/microservices/adminService/middle"

	"github.com/gin-gonic/gin"
)

type EmployeeRouter struct{ service service.IEmployeeService }

func (er *EmployeeRouter) InitApiRouter(router *gin.RouterGroup) {
	publicRouter := router.Group("employee")
	privateRouter := router.Group("employee")
	// 私有路由使用jwt验证
	privateRouter.Use(middle.VerifyJWTAdmin())
	// 依赖注入
	er.service = service.NewEmployeeService(
		dao.NewEmployeeDao(global.DB),
	)
	employeeCtl := controller.NewEmployeeController(er.service)
	{
		publicRouter.POST("/login", employeeCtl.Login)
		privateRouter.POST("/logout", employeeCtl.Logout)
		privateRouter.POST("", employeeCtl.AddEmployee)
		privateRouter.POST("/status/:status", employeeCtl.OnOrOff)
		privateRouter.PUT("/editPassword", employeeCtl.EditPassword)
		privateRouter.PUT("", employeeCtl.UpdateEmployee)
		privateRouter.GET("/page", employeeCtl.PageQuery)
		privateRouter.GET("/:id", employeeCtl.GetById)
	}
}
