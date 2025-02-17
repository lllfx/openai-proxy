package adapter

import (
	"encoding/json"

	"github.com/pkg/errors"
	openai "github.com/sashabaranov/go-openai"
)

type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents a request structure for chat completion API.
type ChatCompletionRequest struct {
	Model       string                  `json:"model" binding:"required"`
	Messages    []ChatCompletionMessage `json:"messages" binding:"required,min=1"`
	MaxTokens   int32                   `json:"max_tokens" binding:"omitempty"`
	Temperature float32                 `json:"temperature" binding:"omitempty"`
	TopP        float32                 `json:"top_p" binding:"omitempty"`
	N           int32                   `json:"n" binding:"omitempty"`
	Stream      bool                    `json:"stream" binding:"omitempty"`
	Stop        []string                `json:"stop,omitempty"`
}

func (req *ChatCompletionRequest) ToGenaiMessages() ([]openai.ChatCompletionMessage, error) {
	if req.Model == TextEmbeddingBgeM3 {
		return nil, errors.New("Chat Completion is not supported for embedding model")
	}

	return req.toVisionGenaiContent()
}

func (req *ChatCompletionRequest) toVisionGenaiContent() ([]openai.ChatCompletionMessage, error) {
	messages := make([]openai.ChatCompletionMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    message.Role,
			Content: message.Content,
			//Refusal:       message.Refusal,
			//MultiContent:  message.MultiContent,
			//Name:         message.Name,
			//FunctionCall:  message.FunctionCall,
			//ToolCalls:     message.ToolCalls,
			//ToolCallID:   message.ToolCallID,
		})
	}
	return messages, nil
}

type CompletionChoice struct {
	Index int `json:"index"`
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
}

type StringArray []string

// UnmarshalJSON implements the json.Unmarshaler interface for StringArray.
func (s *StringArray) UnmarshalJSON(data []byte) error {
	// Check if the data is a JSON array
	if data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}

	// Check if the data is a JSON string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*s = StringArray{str} // Wrap the string in a slice
	return nil
}

// EmbeddingRequest represents a request structure for embeddings API.
type EmbeddingRequest struct {
	Model          string   `json:"model" binding:"required"`
	Messages       []string `json:"input" binding:"required,min=1"`
	EncodingFormat string   `json:"encoding_format,omitempty"`
}

func (req *EmbeddingRequest) ToGenaiMessages() (*openai.EmbeddingRequest, error) {
	if req.Model != TextEmbeddingBgeM3 {
		return nil, errors.New("Embedding is not supported for embedding model " + req.Model)
	}

	return &openai.EmbeddingRequest{
		Input:          req.Messages,
		Model:          openai.EmbeddingModel(req.Model),
		EncodingFormat: openai.EmbeddingEncodingFormat(req.EncodingFormat),
	}, nil
}
