package KCES

import "errors"

// ErrConversionOutputLimitExceeded 表示 KCES 转换结果超过调用方提供的固定宽度字节上限 / ErrConversionOutputLimitExceeded indicates that KCES conversion output exceeds the caller-provided fixed-width byte limit
var ErrConversionOutputLimitExceeded = errors.New("KCES conversion output limit exceeded")
