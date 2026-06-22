package constant

var StopFinishReason = "stop"
var StreamObject = "chat.completion.chunk"
var NonStreamObject = "chat.completion"

// ScannerBufferSize Scanner 缓冲区大小配置（用于流式响应解析）
const ScannerBufferInitial = 1024 * 1024       // 1MB 初始缓冲区
const ScannerBufferMax = 10 * 1024 * 1024      // 10MB 最大缓冲区
