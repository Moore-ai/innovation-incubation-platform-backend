package errcode

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

func New(code int, msg string) *AppError {
	return &AppError{Code: code, Message: msg}
}

func (e *AppError) WithMsg(msg string) *AppError {
	return &AppError{Code: e.Code, Message: msg}
}

var (
	Success          = New(0, "success")

	ErrInvalidParams = New(10001, "参数错误")
	ErrNotFound      = New(10002, "资源不存在")
	ErrDuplicate     = New(10003, "数据已存在")

	ErrUnauthorized  = New(10101, "未登录或Token过期")
	ErrForbidden     = New(10102, "无权限访问")
	ErrRateLimited   = New(10103, "请求过于频繁，请稍后再试")

	ErrStatusInvalid  = New(10201, "当前状态不允许此操作")
	ErrApprovalFailed = New(10202, "审核操作失败")

	ErrAIService = New(10301, "AI服务暂不可用")
	ErrAITimeout = New(10302, "AI服务超时")

	ErrInternal = New(50000, "服务器内部错误")
)
