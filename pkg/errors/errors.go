package errors

// ========== 错误码常量定义 ==========

// CodeSuccess 成功码
const (
	CodeSuccess = 200
)

// HTTP层错误码 (400-599)
const (
	CodeInvalidParam = 400
	CodeUnauthorized = 401
	CodeForbidden    = 403
	CodeNotFound     = 404
	CodeServerError  = 500
)
