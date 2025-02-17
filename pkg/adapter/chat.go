package adapter

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/lllfx/openai-proxy/pkg/util"
	openai "github.com/sashabaranov/go-openai"
)

const (
	genaiRoleUser  = "user"
	genaiRoleModel = "model"
)

type GeminiAdapter struct {
	client *openai.Client
	model  string
}

func NewGeminiAdapter(client *openai.Client, model string) *GeminiAdapter {
	return &GeminiAdapter{
		client: client,
		model:  model,
	}
}

func (g *GeminiAdapter) GenerateContent(
	ctx context.Context,
	req *ChatCompletionRequest,
	messages []openai.ChatCompletionMessage,
) (openai.ChatCompletionResponse, error) {
	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		fmt.Printf("Completion error: %v\n", err)
		return openai.ChatCompletionResponse{}, err
	}
	return resp, nil
}

type ChatCompletionStreamResponse struct {
	Data openai.ChatCompletionStreamResponse
	Err  *openai.APIError
}

func (g *GeminiAdapter) GenerateStreamContent(
	ctx context.Context,
	req *ChatCompletionRequest,
	messages []openai.ChatCompletionMessage,
) (<-chan ChatCompletionStreamResponse, error) {
	stream, err := g.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		fmt.Printf("Completion error: %v\n", err)
		return nil, err
	}
	dataChan := make(chan ChatCompletionStreamResponse)
	go handleStreamIter(stream, dataChan)
	return dataChan, nil
}

func handleStreamIter(stream *openai.ChatCompletionStream, dataChan chan ChatCompletionStreamResponse) {
	defer stream.Close()
	defer close(dataChan)
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Println("\nStream finished")
			break
		}
		if err != nil {
			log.Printf("genai get stream message error %v\n", err)

			dataChan <- ChatCompletionStreamResponse{
				Data: openai.ChatCompletionStreamResponse{},
				Err: &openai.APIError{
					Code:    http.StatusInternalServerError,
					Message: err.Error(),
				},
			}
			break
		}
		dataChan <- ChatCompletionStreamResponse{
			Data: response,
			Err:  nil,
		}
		if len(response.Choices) > 0 && response.Choices[0].FinishReason != "" {
			break
		}
	}
}

func genaiResponseToStreamCompletionResponse(
	model string,
	genaiResp *genai.GenerateContentResponse,
	respID string,
	created int64,
) *CompletionResponse {
	resp := CompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", respID),
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   GetMappedModel(model),
		Choices: make([]CompletionChoice, 0, len(genaiResp.Candidates)),
	}

	for i, candidate := range genaiResp.Candidates {
		var content string
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			if s, ok := candidate.Content.Parts[0].(genai.Text); ok {
				content = string(s)
			}
		}

		choice := CompletionChoice{
			Index: i,
		}
		choice.Delta.Content = content

		if candidate.FinishReason > genai.FinishReasonStop {
			log.Printf("genai message finish reason %s\n", candidate.FinishReason.String())
			openaiFinishReason := string(convertFinishReason(candidate.FinishReason))
			choice.FinishReason = &openaiFinishReason
		}

		resp.Choices = append(resp.Choices, choice)
	}
	return &resp
}

func genaiResponseToOpenaiResponse(
	model string, genaiResp *genai.GenerateContentResponse,
) openai.ChatCompletionResponse {
	resp := openai.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", util.GetUUID()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   GetMappedModel(model),
		Choices: make([]openai.ChatCompletionChoice, 0, len(genaiResp.Candidates)),
	}

	for i, candidate := range genaiResp.Candidates {
		var content string
		if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
			if s, ok := candidate.Content.Parts[0].(genai.Text); ok {
				content = string(s)
			}
		}

		choice := openai.ChatCompletionChoice{
			Index: i,
			Message: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: content,
			},
			FinishReason: convertFinishReason(candidate.FinishReason),
		}
		resp.Choices = append(resp.Choices, choice)
	}
	return resp
}

func convertFinishReason(reason genai.FinishReason) openai.FinishReason {
	openaiFinishReason := openai.FinishReasonStop
	switch reason {
	case genai.FinishReasonMaxTokens:
		openaiFinishReason = openai.FinishReasonLength
	case genai.FinishReasonSafety, genai.FinishReasonRecitation:
		openaiFinishReason = openai.FinishReasonContentFilter
	}
	return openaiFinishReason
}

func setGenaiChatHistory(cs *genai.ChatSession, messages []*genai.Content) {
	cs.History = make([]*genai.Content, 0, len(messages))
	if len(messages) > 1 {
		cs.History = append(cs.History, messages[:len(messages)-1]...)
	}

	if len(cs.History) != 0 && cs.History[len(cs.History)-1].Role != genaiRoleModel {
		cs.History = append(cs.History, &genai.Content{
			Parts: []genai.Part{
				genai.Text(""),
			},
			Role: genaiRoleModel,
		})
	}
}

func setGenaiModelByOpenaiRequest(model *genai.GenerativeModel, req *ChatCompletionRequest) {
	if req.MaxTokens != 0 {
		model.MaxOutputTokens = &req.MaxTokens
	}
	if req.Temperature != 0 {
		model.Temperature = &req.Temperature
	}
	if req.TopP != 0 {
		model.TopP = &req.TopP
	}
	if len(req.Stop) != 0 {
		model.StopSequences = req.Stop
	}
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
	}
}

func (g *GeminiAdapter) GenerateEmbedding(
	ctx context.Context,
	queryReq *openai.EmbeddingRequest,
) (openai.EmbeddingResponse, error) {
	// Create an embedding for the user query
	queryResponse, err := g.client.CreateEmbeddings(ctx, queryReq)
	if err != nil {
		log.Fatal("Error creating query embedding:", err)
		return openai.EmbeddingResponse{}, err
	}
	return queryResponse, nil
}
