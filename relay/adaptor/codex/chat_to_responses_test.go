package codex

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/tidwall/gjson"
)

// parseCompletedOutput 从 generateCompletedEvents 的返回中提取 response.completed 事件，
// 再解析其中的 output 数组。
func parseCompletedOutput(events []string) []interface{} {
	for _, evt := range events {
		if len(evt) < 6 {
			continue
		}
		// SSE 格式: event: response.completed\ndata: {...}\n\n
		// 查找 data: 后面的 JSON
		dataPrefix := "data: "
		idx := indexOf(evt, dataPrefix)
		if idx < 0 {
			continue
		}
		dataStr := evt[idx+len(dataPrefix):]
		dataStr = trimSpace(dataStr)

		if !gjson.Valid(dataStr) {
			continue
		}
		resp := gjson.Parse(dataStr)
		if resp.Get("type").String() == "response.completed" {
			output := resp.Get("response.output")
			if output.Exists() && output.IsArray() {
				var result []interface{}
				json.Unmarshal([]byte(output.Raw), &result)
				return result
			}
			return nil
		}
	}
	return nil
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// sendChunks 模拟发送一系列 SSE chunk 并返回最终完成事件
func sendChunks(chunks []string, fallback bool) []string {
	var param any
	var allEvents []string
	for _, chunk := range chunks {
		events := ConvertOpenAIChatToResponses(nil, nil, []byte(chunk), &param, fallback)
		allEvents = append(allEvents, events...)
	}
	return allEvents
}

func TestConvertOpenAIChatToResponses_FallbackReasoning(t *testing.T) {
	Convey("ConvertOpenAIChatToResponses 的 reasoning 兜底逻辑", t, func() {

		Convey("T1: 仅 reasoning_content 无 content，开关=true → output 含 reasoning + message", func() {
			chunks := []string{
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"Step 1思考"},"finish_reason":null}]}`,
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"reasoning_content":" Step 2思考"},"finish_reason":null}]}`,
				`data: [DONE]`,
			}
			events := sendChunks(chunks, true)
			output := parseCompletedOutput(events)

			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 2)

			// 第一个是 reasoning
			item0, ok := output[0].(map[string]interface{})
			So(ok, ShouldBeTrue)
			So(item0["type"], ShouldEqual, "reasoning")

			// 第二个是 message（兜底）
			item1, ok := output[1].(map[string]interface{})
			So(ok, ShouldBeTrue)
			So(item1["type"], ShouldEqual, "message")
			So(item1["role"], ShouldEqual, "assistant")
			content, ok := item1["content"].([]interface{})
			So(ok, ShouldBeTrue)
			So(len(content), ShouldEqual, 1)
			content0, ok := content[0].(map[string]interface{})
			So(ok, ShouldBeTrue)
			So(content0["type"], ShouldEqual, "output_text")
			So(content0["text"], ShouldEqual, "Step 1思考 Step 2思考")
		})

		Convey("T2: 仅 reasoning_content 无 content，开关=false → output 只有 reasoning", func() {
			chunks := []string{
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"思考中"},"finish_reason":null}]}`,
				`data: [DONE]`,
			}
			events := sendChunks(chunks, false)
			output := parseCompletedOutput(events)

			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 1)
			item0, ok := output[0].(map[string]interface{})
			So(ok, ShouldBeTrue)
			So(item0["type"], ShouldEqual, "reasoning")
		})

		Convey("T3: 正常 reasoning + content，开关=true → output 含 reasoning + message（原行为）", func() {
			chunks := []string{
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"思考中"},"finish_reason":null}]}`,
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"content":" World"},"finish_reason":"stop"}]}`,
				`data: [DONE]`,
			}
			events := sendChunks(chunks, true)
			output := parseCompletedOutput(events)

			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 2)
			So(output[0].(map[string]interface{})["type"], ShouldEqual, "reasoning")
			So(output[1].(map[string]interface{})["type"], ShouldEqual, "message")
			// 消息内容来自 content，不是 reasoning 兜底
			content := output[1].(map[string]interface{})["content"].([]interface{})
			text := content[0].(map[string]interface{})["text"].(string)
			So(text, ShouldEqual, "Hello World")
		})

		Convey("T4: 仅 content 无 reasoning，开关=true → output 只有 message（不触发兜底）", func() {
			chunks := []string{
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}`,
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"content":" there"},"finish_reason":"stop"}]}`,
				`data: [DONE]`,
			}
			events := sendChunks(chunks, true)
			output := parseCompletedOutput(events)

			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 1)
			So(output[0].(map[string]interface{})["type"], ShouldEqual, "message")
		})

		Convey("T5: 仅 reasoning，开关=true，流式事件序列完整", func() {
			chunks := []string{
				`data: {"id":"resp_test","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"思考过程"},"finish_reason":null}]}`,
				`data: [DONE]`,
			}
			events := sendChunks(chunks, true)

			// 收集所有事件类型
			var eventTypes []string
			for _, evt := range events {
				if len(evt) < 6 {
					continue
				}
				if idx := indexOf(evt, "event: "); idx >= 0 {
					endIdx := indexOf(evt[idx+7:], "\n")
					if endIdx >= 0 {
						eventTypes = append(eventTypes, evt[idx+7:idx+7+endIdx])
					}
				}
			}

			// 预期的事件序列
			expected := []string{
				"response.created",
				"response.in_progress",
				"response.output_item.added",
				"response.reasoning_summary_part.added",
				"response.reasoning_summary_text.delta",
				"response.reasoning_summary_text.done",
				"response.reasoning_summary_part.done",
				"response.output_item.done",
				// 以下是兜底 message 事件
				"response.output_item.added",
				"response.content_part.added",
				"response.output_text.delta",
				"response.output_text.done",
				"response.content_part.done",
				"response.output_item.done",
				"response.completed",
			}

			So(eventTypes, ShouldResemble, expected)

			// 验证 output 数组
			output := parseCompletedOutput(events)
			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 2)
			So(output[0].(map[string]interface{})["type"], ShouldEqual, "reasoning")
			So(output[1].(map[string]interface{})["type"], ShouldEqual, "message")
		})
	})
}

func TestSendChunksEmpty(t *testing.T) {
	// 辅助测试：确保辅助函数正常工作
	Convey("sendChunks with empty input", t, func() {
		events := sendChunks([]string{}, true)
		So(len(events), ShouldEqual, 0)
	})
}

// ensure fmt import is used (for the format strings in fallback code)
var _ = fmt.Sprintf
