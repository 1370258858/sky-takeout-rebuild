# 下单到商品到物流调用流程图

下面基于当前代码实现整理，重点覆盖「下单 -> 支付 -> 派送」主链路，并按 API => MQ/Redis/RPC => DB 分层。

## 1) 分层总览（API => MQ/Redis/RPC => DB）

```mermaid
flowchart LR
	%% ========== L1 API ==========
	subgraph L1[顶层 API 层]
		U[用户/前端]
		OAPI[orderService HTTP API\nPOST /order/create\nPOST /order/pay/:id\nPOST /order/pay_timeout/:id]
		GAPI[goodsService HTTP API\nGET /goods/*]
		DAPI[deliveryService HTTP API\n/delivery/*]
	end

	%% ========== L2 Middleware/RPC ==========
	subgraph L2[中间层 MQ / Redis / RPC]
		MQ[(RabbitMQ\norder.pay.timeout.exchange\norder.pay.timeout.queue)]
		REDIS[(Redis\n服务资源初始化\n当前主链路未直接读写)]
		RPCOG[gRPC order -> goods\nGetGoodById]
		RPCOD[gRPC order -> delivery\nCreateDelivery\nGetDeliveryByOrderId\nUpdateDeliveryByOrderId]
	end

	%% ========== L3 DB ==========
	subgraph L3[数据层 DB]
		ODB[(MySQL\norders / order_cart)]
		GDB[(MySQL\ndish + flavors)]
		DDB[(MySQL\ndelivery)]
	end

	U --> OAPI
	U --> GAPI
	U --> DAPI

	OAPI -->|创建订单前校验商品| RPCOG
	RPCOG --> GDB

	OAPI -->|创建订单| ODB
	OAPI -->|发布3分钟延时消息| MQ
	MQ -->|消费后触发超时关单| OAPI

	OAPI -->|支付成功后创建配送单| RPCOD
	RPCOD --> DDB

	OAPI -. 可扩展: 唯一ID/锁 .-> REDIS
	GAPI --> GDB
	DAPI --> DDB
```

## 2) 主流程时序图（下单 -> 支付 -> 派送）

```mermaid
sequenceDiagram
	autonumber
	participant C as Client
	participant O as orderService(API/Service)
	participant G as goodsService(gRPC)
	participant MQ as RabbitMQ(延时队列)
	participant D as deliveryService(gRPC)
	participant ODB as orderDB
	participant GDB as goodsDB
	participant DDB as deliveryDB

	C->>O: POST /order/create
	loop 每个 goodId
		O->>G: GetGoodById(id)
		G->>GDB: SELECT dish/flavors by id
		GDB-->>G: dish detail
		G-->>O: good exists
	end
	O->>ODB: INSERT order(status=待支付, pay_status=未支付)
	ODB-->>O: order created
	O->>MQ: Publish 延时消息(x-delay=3min, orderId)
	O-->>C: 返回下单成功

	alt 3分钟内支付成功
		C->>O: POST /order/pay/:id (payStatus=success)
		O->>ODB: UPDATE order(pay_status=已支付, status=待接单)
		O->>D: CreateDelivery(orderId, deliveryNo, address,...)
		D->>DDB: INSERT delivery(status=1 呼叫骑手中)
		DDB-->>D: delivery created
		D-->>O: ok
		O-->>C: 支付成功，进入配送流程
	else 支付超时未完成
		MQ-->>O: consume timeout message
		O->>ODB: 查询订单支付状态
		alt 未支付
			O->>ODB: UPDATE order(status=已取消, cancel_reason=pay timeout)
			O-->>C: 订单超时取消
		else 已支付
			O-->>C: 忽略超时消息
		end
	end
```

## 3) 补充说明

- 订单号当前由 orderService 本地 Snowflake 生成（非 Redis 自增）。
- Redis 在三个服务中已完成资源初始化，但在上述主链路里没有直接读写语句。
- 退款链路会调用 deliveryService：GetDeliveryByOrderId / UpdateDeliveryByOrderId，并更新订单为 refunded（不属于本次“下单->支付->派送”主路径）。
