package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/common/render"
	"github.com/songquanpeng/one-api/relay/meta"
	"github.com/songquanpeng/one-api/relay/model"
)

const (
	dataPrefix        = "data: "
	eventPrefix       = "event: "
	done              = "[DONE]"
	dataPrefixLength  = len(dataPrefix)
	eventPrefixLength = len(eventPrefix)
)

var ModelList = []string{
	"gpt-4o",
	"gpt-4o-mini",
	"gpt-4-turbo",
	"gpt-4",
	"gpt-3.5-turbo",
	"o1",
	"o1-mini",
}

func DoResponsesResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (*model.Usage, *model.ErrorWithStatusCode) {
	var textResponse model.ResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Log.Errorf("[%s] %+v", "read_response_body_failed", err)
		return nil, ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	err = resp.Body.Close()
	if err != nil {
		logger.Log.Errorf("[%s] %+v", "close_response_body_failed", err)
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	err = json.Unmarshal(responseBody, &textResponse)
	if err != nil {
		logger.Log.Errorf("[%s] %+v", "invalid_json_response", err)
		return nil, ErrorWrapper(err, "invalid_json_response", http.StatusInternalServerError)
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

	for k, v := range resp.Header {
		for _, vv := range v {
			c.Writer.Header().Add(k, vv)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		logger.Log.Errorf("[%s] %+v", "copy_response_body_failed", err)
		return nil, ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	err = resp.Body.Close()
	if err != nil {
		logger.Log.Errorf("[%s] %+v", "close_response_body_failed", err)
		return nil, ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	usage := &model.Usage{
		PromptTokens:     textResponse.Usage.InputTokens,
		CompletionTokens: textResponse.Usage.OutputTokens,
		TotalTokens:      textResponse.Usage.TotalTokens,
	}

	c.Set(ctxkey.ResponseBody, string(responseBody))
	return usage, nil
}

func StreamResponsesHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, string, *model.Usage) {
	responseText := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	var usage *model.Usage
	capture := model.ResponsesStreamCapture{}
	var currentFrame *model.ResponsesStreamFrame
	var deltaText strings.Builder
	var deltaFrame *model.ResponsesStreamFrame
	var pendingEvent string
	var outputItems []model.ResponsesItem
	var outputItemByID = map[string]int{}
	var skippedItemIDs = map[string]struct{}{}

	flushFrame := func() {
		if currentFrame != nil {
			capture.Frames = append(capture.Frames, *currentFrame)
			currentFrame = nil
		}
	}
	flushDeltaFrame := func() {
		if deltaFrame != nil {
			payload := map[string]any{
				"type":  "response.output_text.delta",
				"delta": deltaText.String(),
			}
			if data, err := json.Marshal(payload); err == nil {
				deltaFrame.Data = json.RawMessage(data)
				capture.Frames = append(capture.Frames, *deltaFrame)
			}
			deltaFrame = nil
			deltaText.Reset()
		}
	}

	common.SetEventStreamHeaders(c)

	doneRendered := false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flushFrame()
			continue
		}
		if strings.HasPrefix(line, eventPrefix) {
			flushFrame()
			pendingEvent = strings.TrimSpace(line[eventPrefixLength:])
			continue
		}
		if len(line) < dataPrefixLength || line[:dataPrefixLength] != dataPrefix {
			continue
		}

		payload := line[dataPrefixLength:]
		if strings.HasPrefix(payload, done) {
			flushDeltaFrame()
			if currentFrame == nil {
				currentFrame = &model.ResponsesStreamFrame{Event: pendingEvent}
			}
			if currentFrame.Event == "" {
				currentFrame.Event = pendingEvent
			}
			currentFrame.Data = json.RawMessage(`"[DONE]"`)
			currentFrame.Done = true
			pendingEvent = ""
			render.StringData(c, line)
			doneRendered = true
			continue
		}

		var streamResponse model.ResponsesStreamEvent
		err := json.Unmarshal([]byte(payload), &streamResponse)
		if err != nil {
			logger.Log.Errorf("error unmarshalling stream response: " + err.Error())
			render.StringData(c, line)
			continue
		}
		render.StringData(c, line)

		if streamResponse.Type == "response.output_text.delta" {
			if streamResponse.Delta != nil {
				if s, ok := streamResponse.Delta.(string); ok {
					deltaText.WriteString(s)
				}
			}
			if deltaFrame == nil {
				deltaFrame = &model.ResponsesStreamFrame{Event: streamResponse.Type}
			}
			pendingEvent = ""
			continue
		}
		if strings.HasSuffix(streamResponse.Type, ".delta") {
			pendingEvent = ""
			continue
		}

		flushDeltaFrame()

		if currentFrame == nil {
			currentFrame = &model.ResponsesStreamFrame{Event: pendingEvent}
		}
		if currentFrame.Event == "" {
			currentFrame.Event = pendingEvent
		}
		currentFrame.Data = json.RawMessage(payload)
		pendingEvent = ""

		if streamResponse.Type == "response.output_item.added" || streamResponse.Type == "response.output_item.done" {
			if streamResponse.Item == nil {
				continue
			}
			itemID := streamResponse.Item.ID
			if itemID == "" {
				itemID = streamResponse.Item.CallID
			}
			if itemID == "" {
				logger.Log.Errorf("skip output item without id")
				continue
			}
			if !shouldKeepResponsesOutputItem(streamResponse.Item.Type) {
				skippedItemIDs[itemID] = struct{}{}
				continue
			}
			if _, skipped := skippedItemIDs[itemID]; skipped {
				continue
			}
			if streamResponse.Type == "response.output_item.added" {
				outputItemByID[itemID] = len(outputItems)
				outputItems = append(outputItems, *streamResponse.Item)
			} else if idx, ok := outputItemByID[itemID]; ok && idx < len(outputItems) {
				outputItems[idx] = *streamResponse.Item
			} else {
				outputItems = append(outputItems, *streamResponse.Item)
			}
		}

		if streamResponse.Usage != nil {
			capture.Usage = streamResponse.Usage
			usage = &model.Usage{
				PromptTokens:     streamResponse.Usage.InputTokens,
				CompletionTokens: streamResponse.Usage.OutputTokens,
				TotalTokens:      streamResponse.Usage.TotalTokens,
			}
		}

		if streamResponse.Response != nil && streamResponse.Response.Usage.TotalTokens > 0 {
			capture.Usage = &streamResponse.Response.Usage
			usage = &model.Usage{
				PromptTokens:     streamResponse.Response.Usage.InputTokens,
				CompletionTokens: streamResponse.Response.Usage.OutputTokens,
				TotalTokens:      streamResponse.Response.Usage.TotalTokens,
			}
		}

		if streamResponse.Type == "response.completed" && streamResponse.Response != nil {
			resp := streamResponse.Response
			if len(resp.Output) == 0 && len(outputItems) > 0 {
				resp.Output = outputItems
			}
			capture.Response = resp
		}

		// frame is flushed when the blank line arrives
	}

	if err := scanner.Err(); err != nil {
		logger.Log.Errorf("error reading stream: " + err.Error())
	}

	if !doneRendered {
		render.Done(c)
	}

	flushFrame()
	flushDeltaFrame()
	if len(capture.Frames) > 0 || capture.Response != nil {
		if capture.Response != nil {
			if capture.Usage == nil {
				capture.Usage = &capture.Response.Usage
			}
			if capture.Usage != nil {
				capture.Response.Usage = *capture.Usage
			}
		}
		if respJSON, err := json.Marshal(capture); err == nil {
			c.Set(ctxkey.ResponseBody, string(respJSON))
		}
	}

	err := resp.Body.Close()
	if err != nil {
		logger.Log.Errorf("[%s] %+v", "close_response_body_failed", err)
		return ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), "", nil
	}

	return nil, responseText, usage
}

func shouldKeepResponsesOutputItem(itemType string) bool {
	switch itemType {
	case "message", "reasoning", "function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output", "tool_search_call":
		return true
	default:
		return false
	}
}

func ErrorWrapper(err error, code string, statusCode int) *model.ErrorWithStatusCode {
	return &model.ErrorWithStatusCode{
		Error: model.Error{
			Message: err.Error(),
			Type:    "one_api_error",
			Param:   "",
			Code:    code,
		},
		StatusCode: statusCode,
	}
}

// appendToFile 追加内容到文件（文件不存在则创建）
func AppendToFile(filename string, content string) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, err = f.WriteString(content)
	if err != nil {
		fmt.Println("追加文件报错", filename, err)
	}
}
