package elog

import (
	"github.com/ez4bk/ezboot"
	"github.com/pkg/errors"
)

// Warnw 发出一条警告消息并将错误包装后返回
func Warnw(log *ezboot.Logger, err error, message string, args ...any) error {
	if err != nil {
		log.Warnw(message, append([]any{"error", err}, args...)...)
		return errors.Wrap(err, message)
	}
	return nil
}

// Errorw 发出一条错误消息并将错误包装后返回
func Errorw(log *ezboot.Logger, err error, message string, args ...any) error {
	if err != nil {
		log.Errorw(message, append([]any{"error", err}, args...)...)
		return errors.Wrap(err, message)
	}
	return nil
}
