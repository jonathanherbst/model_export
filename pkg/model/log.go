package model

import (
	"log/slog"
	"reflect"
)

var logger = SetLogger(slog.Default())

func SetLogger(logger *slog.Logger) *slog.Logger {
	return logger.WithGroup(reflect.TypeOf(token{}).PkgPath())
}

type token struct{}
