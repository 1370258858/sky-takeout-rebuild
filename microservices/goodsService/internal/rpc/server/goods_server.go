package server

import (
	"context"
	"errors"

	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/model"
	goodsv1 "sky-takeout/microservices/goodsService/internal/rpc/pb"

	"gorm.io/gorm"
)

// GoodsRPCServer implements goods gRPC methods for downstream callers.
type GoodsRPCServer struct {
	goodsv1.UnimplementedGoodsServiceServer
}

func NewGoodsRPCServer() *GoodsRPCServer {
	return &GoodsRPCServer{}
}

func (s *GoodsRPCServer) GetSku(ctx context.Context, req *goodsv1.GetSkuRequest) (*goodsv1.GetSkuResponse, error) {
	var dish model.Dish
	err := global.DB.WithContext(ctx).Where("id = ?", req.GetSkuId()).First(&dish).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &goodsv1.GetSkuResponse{
				SkuId:     req.GetSkuId(),
				Name:      "",
				PriceCent: 0,
				Available: false,
			}, nil
		}
		return nil, err
	}

	return &goodsv1.GetSkuResponse{
		SkuId:     int64(dish.Id),
		Name:      dish.Name,
		PriceCent: int64(dish.Price * 100),
		Available: dish.Status == 1,
	}, nil
}

func (s *GoodsRPCServer) ListHotGoods(ctx context.Context, req *goodsv1.ListHotGoodsRequest) (*goodsv1.ListHotGoodsResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var dishes []model.Dish
	if err := global.DB.WithContext(ctx).Order("id desc").Limit(limit).Find(&dishes).Error; err != nil {
		return nil, err
	}

	items := make([]*goodsv1.HotGoodsItem, 0, len(dishes))
	for _, d := range dishes {
		items = append(items, &goodsv1.HotGoodsItem{
			SkuId:     int64(d.Id),
			Name:      d.Name,
			SoldCount: 0,
		})
	}

	return &goodsv1.ListHotGoodsResponse{Items: items}, nil
}
