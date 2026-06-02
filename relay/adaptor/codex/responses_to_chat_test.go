package codex

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// parseOutputArray 从 ConvertChatResponseToResponses 的结果中提取 output 数组
func parseOutputArray(respBytes []byte) []interface{} {
	var resp map[string]interface{}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil
	}
	if output, ok := resp["output"].([]interface{}); ok {
		return output
	}
	return nil
}

func TestConvertChatResponseToResponses_FallbackReasoning(t *testing.T) {
	Convey("ConvertChatResponseToResponses 的 reasoning 兜底逻辑（非流式）", t, func() {

		Convey("T1(非流式): 仅 reasoning_content 无 content，开关=true → output 含 reasoning + message", func() {
			chatResp := map[string]interface{}{
				"id":      "chat_test",
				"created": 1700000000,
				"model":   "deepseek-flash",
				"choices": []interface{}{
					map[string]interface{}{
						"index": 0,
						"message": map[string]interface{}{
							"role":             "assistant",
							"reasoning_content": "深度思考过程",
						},
						"finish_reason": "stop",
					},
				},
			}
			chatBody, _ := json.Marshal(chatResp)

			result := ConvertChatResponseToResponses(chatBody, "deepseek-flash", true)
			output := parseOutputArray(result)

			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 2)
			So(output[0].(map[string]interface{})["type"], ShouldEqual, "reasoning")
			So(output[1].(map[string]interface{})["type"], ShouldEqual, "message")
			So(output[1].(map[string]interface{})["role"], ShouldEqual, "assistant")
			content := output[1].(map[string]interface{})["content"].([]interface{})
			So(content[0].(map[string]interface{})["text"], ShouldEqual, "深度思考过程")
		})

		Convey("T2(非流式): 仅 reasoning_content 无 content，开关=false → output 只含 reasoning", func() {
			chatResp := map[string]interface{}{
				"id":      "chat_test",
				"created": 1700000000,
				"model":   "deepseek-flash",
				"choices": []interface{}{
					map[string]interface{}{
						"index": 0,
						"message": map[string]interface{}{
							"role":             "assistant",
							"reasoning_content": "深度思考过程",
						},
						"finish_reason": "stop",
					},
				},
			}
			chatBody, _ := json.Marshal(chatResp)

			result := ConvertChatResponseToResponses(chatBody, "deepseek-flash", false)
			output := parseOutputArray(result)

			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 1)
			So(output[0].(map[string]interface{})["type"], ShouldEqual, "reasoning")
		})

		Convey("T3(非流式): 正常 reasoning + content，开关=true → output 含 reasoning + message（原行为）", func() {
			chatResp := map[string]interface{}{
				"id":      "chat_test",
				"created": 1700000000,
				"model":   "deepseek-flash",
				"choices": []interface{}{
					map[string]interface{}{
						"index": 0,
						"message": map[string]interface{}{
							"role":             "assistant",
							"content":          "Hello World",
							"reasoning_content": "思考中",
						},
						"finish_reason": "stop",
					},
				},
			}
			chatBody, _ := json.Marshal(chatResp)

			result := ConvertChatResponseToResponses(chatBody, "deepseek-flash", true)
			output := parseOutputArray(result)

			So(output, ShouldNotBeNil)
			So(len(output), ShouldEqual, 2)
			So(output[0].(map[string]interface{})["type"], ShouldEqual, "reasoning")
			So(output[1].(map[string]interface{})["type"], ShouldEqual, "message")
			// content 应来自正常 content 字段，不是 reasoning 兜底
			content := output[1].(map[string]interface{})["content"].([]interface{})
			So(content[0].(map[string]interface{})["text"], ShouldEqual, "Hello World")
		})
	})
}
