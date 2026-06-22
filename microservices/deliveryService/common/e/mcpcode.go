package e

// MCP Tool Response Code
const (
	MCPCodeSuccess       = 0   // 成功
	MCPCodeParamError    = 400 // 参数错误
	MCPCodeElicitTimeout = 408 // Elicit 超时
	MCPCodeElicitCancel  = 409 // Elicit 被用户取消
	MCPCodeAuthError     = 401 // 认证错误
	MCPCodeNotFound      = 404 // 资源不存在
	MCPCodeServerError   = 500 // 服务器错误
)

// MCP Tool Error Code
const (
	MCPErrorCodeInvalidParam        = "INVALID_PARAM"         // 参数无效
	MCPErrorCodeMissingParam        = "MISSING_PARAM"         // 缺少必需参数
	MCPErrorCodeElicitTimeout       = "ELICIT_TIMEOUT"        // Elicit 超时
	MCPErrorCodeElicitCancelled     = "ELICIT_CANCELLED"      // Elicit 被取消
	MCPErrorCodeInvalidMode         = "INVALID_MODE"          // 模式错误（如cartId和goodIds混用）
	MCPErrorCodeBusinessLogicError  = "BUSINESS_LOGIC_ERROR"  // 业务逻辑错误
	MCPErrorCodeResourceNotFound    = "RESOURCE_NOT_FOUND"    // 资源不存在
	MCPErrorCodeUnauthorized        = "UNAUTHORIZED"          // 未授权
	MCPErrorCodeInternalServerError = "INTERNAL_SERVER_ERROR" // 内部服务器错误
	MCPErrorCodeRPCError            = "RPC_ERROR"             // RPC 调用错误
	MCPErrorCodeDatabaseError       = "DATABASE_ERROR"        // 数据库错误
)

// MCPToolResponse 统一的 MCP 工具响应结构
type MCPToolResponse struct {
	Code      int    `json:"code"`                 // 响应码 (0=成功，>0=错误)
	Msg       string `json:"msg"`                  // 响应消息
	ErrorCode string `json:"error_code,omitempty"` // 错误码（仅在出错时有值）
	Data      any    `json:"data,omitempty"`       // 响应数据（成功时有值）
}

// NewMCPSuccessResponse 创建成功响应
func NewMCPSuccessResponse(data any) *MCPToolResponse {
	return &MCPToolResponse{
		Code: MCPCodeSuccess,
		Msg:  "success",
		Data: data,
	}
}

// NewMCPErrorResponse 创建错误响应
func NewMCPErrorResponse(code int, msg, errorCode string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      code,
		Msg:       msg,
		ErrorCode: errorCode,
	}
}

// NewMCPParamErrorResponse 参数错误响应
func NewMCPParamErrorResponse(msg string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeParamError,
		Msg:       msg,
		ErrorCode: MCPErrorCodeInvalidParam,
	}
}

// NewMCPMissingParamResponse 缺少参数响应
func NewMCPMissingParamResponse(paramName string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeParamError,
		Msg:       "missing required parameter: " + paramName,
		ErrorCode: MCPErrorCodeMissingParam,
	}
}

// NewMCPElicitTimeoutResponse Elicit 超时响应
func NewMCPElicitTimeoutResponse(msg string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeElicitTimeout,
		Msg:       msg,
		ErrorCode: MCPErrorCodeElicitTimeout,
	}
}

// NewMCPElicitCancelResponse Elicit 被取消响应
func NewMCPElicitCancelResponse() *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeElicitCancel,
		Msg:       "user cancelled the elicitation",
		ErrorCode: MCPErrorCodeElicitCancelled,
	}
}

// NewMCPInvalidModeResponse 模式错误响应
func NewMCPInvalidModeResponse(msg string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeParamError,
		Msg:       msg,
		ErrorCode: MCPErrorCodeInvalidMode,
	}
}

// NewMCPBusinessErrorResponse 业务逻辑错误响应
func NewMCPBusinessErrorResponse(msg string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeServerError,
		Msg:       msg,
		ErrorCode: MCPErrorCodeBusinessLogicError,
	}
}

// NewMCPNotFoundResponse 资源不存在响应
func NewMCPNotFoundResponse(resource string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeNotFound,
		Msg:       resource + " not found",
		ErrorCode: MCPErrorCodeResourceNotFound,
	}
}

// NewMCPServerErrorResponse 服务器错误响应
func NewMCPServerErrorResponse(msg string) *MCPToolResponse {
	return &MCPToolResponse{
		Code:      MCPCodeServerError,
		Msg:       msg,
		ErrorCode: MCPErrorCodeInternalServerError,
	}
}

// ToMap 将响应转换为 map[string]any（用于 MCP SDK 序列化）
func (r *MCPToolResponse) ToMap() map[string]any {
	result := map[string]any{
		"code": r.Code,
		"msg":  r.Msg,
	}
	if r.ErrorCode != "" {
		result["error_code"] = r.ErrorCode
	}
	if r.Data != nil {
		result["data"] = r.Data
	}
	return result
}
