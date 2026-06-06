package client

import (
	pb "sky-takeout/microservices/rpc/pb/goodsv1"

	"google.golang.org/grpc"
)

func NewGoodsRPCClient(conn *grpc.ClientConn) pb.GoodsClient {
	return pb.NewGoodsClient(conn)
}
