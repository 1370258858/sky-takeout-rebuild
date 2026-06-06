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
	Id          uint64    `json:"id" gorm:"primaryKey;AUTO_INCREMENT"`
	Name        string    `json:"name"`
	DishId      uint64    `json:"dishId"`
	Price       float64   `json:"price"`
	Image       string    `json:"image"`
	Description string    `json:"description"`
	Status      int       `json:"status"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
	CreateUser  uint64    `json:"createUser"`
	UpdateUser  uint64    `json:"updateUser"`
	// 一对多
	Flavors []DishFlavor `json:"flavors"`
}

func (Dish) TableName() string {
	return "dish"
}

type DishFlavor struct {
	Id     uint64 `json:"id"`      //口味id
	DishId uint64 `json:"dish_id"` //菜品id
	Name   string `json:"name"`    //口味主题 温度|甜度|辣度
	Value  string `json:"value"`   //口味信息 可多个
}

func (DishFlavor) TableName() string {
	return "dish_flavor"
}
