package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// GoodsRPCClient wraps goods service RPC client usage in order service.
type GoodsRPCClient struct {
	httpBase string
	httpCli  *http.Client
}

func NewGoodsRPCClientFromEnv() (*GoodsRPCClient, error) {
	addr := os.Getenv("GOODS_SERVICE_HTTP_ADDR")
	if strings.TrimSpace(addr) == "" {
		addr = "http://127.0.0.1:18083"
	}

	return &GoodsRPCClient{
		httpBase: strings.TrimRight(addr, "/"),
		httpCli:  &http.Client{},
	}, nil
}

func (c *GoodsRPCClient) Close() error {
	return nil
}

func (c *GoodsRPCClient) ListDishByID(ctx context.Context, dishID int64) (int64, error) {
	url := c.httpBase + "/goods/dish/list?id=" + strconv.FormatInt(dishID, 10)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("goods list http status: %d", resp.StatusCode)
	}

	var payload struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}

	if len(payload.Data) == 0 || string(payload.Data) == "null" {
		return 0, nil
	}

	var rows []map[string]any
	if err := json.Unmarshal(payload.Data, &rows); err != nil {
		var obj map[string]any
		if err2 := json.Unmarshal(payload.Data, &obj); err2 != nil {
			return 0, err
		}
		return 1, nil
	}

	return int64(len(rows)), nil
}
