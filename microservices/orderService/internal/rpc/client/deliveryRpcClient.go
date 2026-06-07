package client

import (
	pb "sky-takeout/microservices/rpc/pb/deliveryv1"

	"google.golang.org/grpc"
)

func NewDeliveryRPCClient(conn *grpc.ClientConn) pb.DeliveryClient {
	return pb.NewDeliveryClient(conn)
}
