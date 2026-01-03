package logger

import (
    "os"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func InitLogger() {
    // Trong môi trường Production, nên dùng `zapcore.NewCore()`
    config := zap.NewDevelopmentConfig()
    Log, _ = config.Build(
        zap.AddCaller(),
    )
}

func Info(msg string, fields ...zap.Field) {
    Log.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
    Log.Error(msg, fields...)
}