package mq

import "errors"

// PermanentError 表示重试也无法修复的消息，例如 JSON 损坏或协议版本不支持。
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string {
	if e == nil || e.Err == nil {
		return "permanent message error"
	}
	return e.Err.Error()
}
func (e *PermanentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// MarkPermanent 把普通错误标记为不可重试错误。
func MarkPermanent(err error) error {
	// 处理器通过该函数告诉 Consumer：当前错误应直接进入 DLQ。
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}
func isPermanent(err error) bool { var target *PermanentError; return errors.As(err, &target) }
