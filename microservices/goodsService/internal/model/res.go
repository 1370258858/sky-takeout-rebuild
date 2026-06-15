package model

import (
	"time"
)

// 这里定义了controller层和service层都需要用到的结构体，避免重复定义
type Resquest struct {
	ID uint64 `json:"id"`
}
type Response struct {
}

type Dish struct {
	Id          uint64    `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Name        string    `json:"name" gorm:"column:name"`
	DishId      uint64    `json:"dishId" gorm:"column:category_id"`
	Price       float64   `json:"price" gorm:"column:price"`
	Image       string    `json:"image" gorm:"column:image"`
	Description string    `json:"description" gorm:"column:description"`
	Status      int       `json:"status" gorm:"column:status"`
	CreateTime  time.Time `json:"createTime" gorm:"column:create_time"`
	UpdateTime  time.Time `json:"updateTime" gorm:"column:update_time"`
	CreateUser  uint64    `json:"createUser" gorm:"column:create_user"`
	UpdateUser  uint64    `json:"updateUser" gorm:"column:update_user"`
	// 一对多
	Flavors []DishFlavor `json:"flavors" gorm:"foreignKey:DishId;references:Id"`
}

func (Dish) TableName() string {
	return "dish"
}
func (DishFlavor) TableName() string {
	return "dish_flavor"
}

type DishFlavor struct {
	Id     uint64 `json:"id" gorm:"column:id;primaryKey;autoIncrement"` //口味id
	DishId uint64 `json:"dish_id" gorm:"column:dish_id"`                //菜品id
	Name   string `json:"name" gorm:"column:name"`                      //口味主题 温度|甜度|辣度
	Value  string `json:"value" gorm:"column:value"`                    //口味信息 可多个
}
