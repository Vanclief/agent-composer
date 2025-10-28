package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/agent-composer/runtime/types"
	"github.com/vanclief/ez"
)

type ChatGPT struct {
	client              *openai.Client
	responsesToMessages map[string]int
}

func NewChatGPT(client *openai.Client) (types.LLMProvider, error) {
	const op = "providers.NewOpenAI"

	chatgpt := &ChatGPT{client: client, responsesToMessages: make(map[string]int)}

	return chatgpt, nil
}

func (provider *ChatGPT) ValidateModel(ctx context.Context, model string) error {
	const op = "ChatGPT.ValidateModel"

	// NOTE: Probably we can get rid of this method, check once we add another
	// LLM provider

	if model == "" {
		return ez.New(op, ez.EINVALID, "model is required", nil)
	}

	// Uses the official SDK's Models service (Get) to verify the model ID.
	// Any 4xx/5xx from the API bubbles up here.
	_, err := provider.client.Models.Get(ctx, model)
	if err != nil {
		errMsg := fmt.Sprintf("ChatGPT model %s does not exist", model)
		return ez.New(op, ez.EINVALID, errMsg, err)
	}
	return nil
}

func (provider *ChatGPT) Chat(ctx context.Context, model string, request *types.ChatRequest) (types.ChatResponse, error) {
	const op = "ChatGPT.Chat"

	originalMessageCount := len(request.Messages)

	// Step 1) Only pass the messages delta if continuing a previous response
	if request.PreviousResponseID != "" {
		lastMsg, ok := provider.responsesToMessages[request.PreviousResponseID]
		if ok {
			if lastMsg <= len(request.Messages) {
				request.Messages = request.Messages[lastMsg:]
			} else {
				request.Messages = request.Messages[:0]
			}
		}
	}

	// Step 2) Create the request
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(model),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: messagesToResponsesInputParam(request.Messages),
		},
	}

	// If it has a prev response ID, set it to continue the thread
	if request.PreviousResponseID != "" {
		params.PreviousResponseID = openai.String(request.PreviousResponseID)
	}

	// If there are are any tool calls create them
	if len(request.Tools) > 0 {
		tools, err := buildFunctionTools(request.Tools)
		if err != nil {
			return types.ChatResponse{}, ez.New(op, ez.EINVALID, "invalid tool definition", err)
		}

		params.Tools = tools
	}

	if isReasoningModel(model) {
		params.Reasoning = shared.ReasoningParam{
			Effort: shared.ReasoningEffort(request.ThinkingEffort), // "low" | "medium" | "high"
		}
	}

	// Step 3) Call the ChatGPT API
	response, err := provider.client.Responses.New(ctx, params)
	if err != nil {
		return types.ChatResponse{}, ez.New(op, ez.EINTERNAL, "Responses API call failed", err)
	}

	// Log token usage
	usage, _ := extractUsage(response)
	totalCost := float64(usage.InputTokens)*0.00000125 + float64(usage.OutputTokens)*0.00001
	log.Info().
		Int64("input_tokens", usage.InputTokens).
		Float64("cost", totalCost).
		Int64("reasoning_tokens", usage.OutputTokensDetails.ReasoningTokens).
		Int64("output_tokens", usage.OutputTokens).
		Int64("total_tokens", usage.TotalTokens).
		Msg("Chat response")

	// Exit 1) If the response is not empty, return it
	if response.OutputText() != "" {
		return types.ChatResponse{
			ID:    response.ID,
			Text:  response.OutputText(),
			Model: model,
		}, nil
	}

	// Step 4) Add the response ID to the map to track messages
	provider.responsesToMessages[response.ID] = originalMessageCount

	// Step 5) If the response is not empty, probably we have tool calls
	var toolCalls []types.ToolCall
	for _, outputItem := range response.Output {
		switch outputItem.Type {
		case "function_call":

			call := outputItem.AsFunctionCall()

			toolCall := types.ToolCall{
				Name:          call.Name,
				CallID:        call.CallID,
				Arguments:     call.Arguments,                  // string
				JSONArguments: json.RawMessage(call.Arguments), // ready to Unmarshal
			}

			toolCalls = append(toolCalls, toolCall)
		case "reasoning":
			log.Info().Msg("Reasoning...")
			reasoning := outputItem.AsReasoning()
			for _, summary := range reasoning.Summary {
				log.Info().
					Msg(summary.Text)
			}

			continue
		default:
			continue
		}
	}

	return types.ChatResponse{
		ID:        response.ID,
		Model:     model,
		ToolCalls: toolCalls,
	}, nil
}

// messagesToResponsesInputParam converts our generic Message slice into the Responses API's
// ResponseInputParam union. It wraps user/system messages as input, and assistant messages as output.
func messagesToResponsesInputParam(messages []types.Message) responses.ResponseInputParam {
	items := make(responses.ResponseInputParam, 0, len(messages))

	for _, m := range messages {
		switch m.Role {

		case types.MessageRoleSystem, types.MessageRoleUser:
			// History is sent as input messages (role: system/user/assistant)
			inText := responses.ResponseInputContentParamOfInputText(m.Content)
			inMsg := responses.ResponseInputItemMessageParam{
				Role:    string(m.Role),
				Content: responses.ResponseInputMessageContentListParam{inText},
			}
			items = append(items, responses.ResponseInputItemUnionParam{OfInputMessage: &inMsg})

		case types.MessageRoleAssistant:
			if m.ToolCall != nil {
				// Persisted function call from the assistant.
				items = append(items, responses.ResponseInputItemParamOfFunctionCall(
					m.ToolCall.Arguments,
					m.ToolCall.CallID,
					m.ToolCall.Name,
				))
			} else {
				// Assistant history must be sent as *output_message* content.
				outText := responses.ResponseOutputTextParam{Text: m.Content}
				outContent := responses.ResponseOutputMessageContentUnionParam{
					OfOutputText: &outText,
				}
				outMsg := responses.ResponseOutputMessageParam{
					Content: []responses.ResponseOutputMessageContentUnionParam{outContent},
					// Role and Type default to assistant/message; OK to omit.
				}
				items = append(items, responses.ResponseInputItemUnionParam{
					OfOutputMessage: &outMsg,
				})
			}

		case types.MessageRoleTool:

			// CRITICAL: send as function_call_output tied to the original call_id
			out := responses.ResponseInputItemFunctionCallOutputParam{
				CallID: m.ToolCallID, // MUST match the model's function_call.call_id
				Output: m.Content,    // JSON string (e.g. "{\"answer\":\"ok\"}" or "\"plain text\"")
				// Type is set by the SDK to "function_call_output" (zero value marshal)

				// Name is optional; call_id is what binds it.
				// Name: m.Name,
			}
			items = append(items, responses.ResponseInputItemUnionParam{
				OfFunctionCallOutput: &out,
			})

		default:
			// ignore or handle other roles
		}
	}
	return items
}

func buildFunctionTools(toolDefs []types.ToolDefinition) ([]responses.ToolUnionParam, error) {
	const op = "ChatGPT.buildFunctionTools"

	var toolParams []responses.ToolUnionParam

	for _, definition := range toolDefs {
		if definition.Name == "" {
			return nil, ez.New(op, ez.EINVALID, "tool name is required", nil)
		}

		parameters := definition.JSONSchema
		if parameters == nil {
			parameters = map[string]any{}
		}

		// Minimal, valid JSON Schema scaffold
		if _, hasType := parameters["type"]; !hasType {
			parameters["type"] = "object"
		}
		if _, hasProps := parameters["properties"]; !hasProps {
			parameters["properties"] = map[string]any{}
		}

		// Keep the human guidance: put the tool description into the schema root.
		// The model reads it even if the top-level Description field isn't sent.
		if definition.Description != "" {
			// Do not overwrite an existing root description if the user provided one.
			if _, hasRootDesc := parameters["description"]; !hasRootDesc {
				parameters["description"] = definition.Description
			}
		}

		// Strict=false to avoid "additionalProperties:false" and "required must include every key".
		unionParam := responses.ToolParamOfFunction(definition.Name, parameters, false)

		toolParams = append(toolParams, unionParam)
	}

	return toolParams, nil
}

// Add somewhere in your package:
type TokenUsage struct {
	InputTokens         int64 `json:"input_tokens"`
	OutputTokens        int64 `json:"output_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

func extractUsage(resp *responses.Response) (TokenUsage, error) {
	// The SDK exposes RawJSON(); usage is in there per API docs.
	var wire struct {
		Usage TokenUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(resp.RawJSON()), &wire); err != nil {
		return TokenUsage{}, err
	}
	return wire.Usage, nil
}

func isReasoningModel(model string) bool {
	modelLower := strings.ToLower(model)
	return strings.HasPrefix(modelLower, "gpt-5") || strings.HasPrefix(modelLower, "o")
}
