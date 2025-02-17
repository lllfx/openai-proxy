package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	openai "github.com/sashabaranov/go-openai"
	"github.com/zhu327/gemini-openai-proxy/pkg/adapter"
	"google.golang.org/api/googleapi"
)

func IndexHandler(c *gin.Context) {
	c.JSON(http.StatusMisdirectedRequest, gin.H{
		"message": "Welcome to the OpenAI API! Documentation is available at https://platform.openai.com/docs/api-reference",
	})
}

func ModelListHandler(c *gin.Context) {
	owner := adapter.GetOwner()
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data": []any{
			openai.Model{
				CreatedAt: 1686935002,
				ID:        adapter.GetModel(openai.GPT3Dot5Turbo),
				Object:    "model",
				OwnedBy:   owner,
			},
			openai.Model{
				CreatedAt: 1686935002,
				ID:        adapter.GetModel(openai.GPT4),
				Object:    "model",
				OwnedBy:   owner,
			},
			openai.Model{
				CreatedAt: 1686935002,
				ID:        adapter.GetModel(openai.GPT4TurboPreview),
				Object:    "model",
				OwnedBy:   owner,
			},
			openai.Model{
				CreatedAt: 1686935002,
				ID:        adapter.GetModel(openai.GPT4VisionPreview),
				Object:    "model",
				OwnedBy:   owner,
			},
			openai.Model{
				CreatedAt: 1686935002,
				ID:        adapter.GetModel(string(openai.AdaEmbeddingV2)),
				Object:    "model",
				OwnedBy:   owner,
			},
			openai.Model{
				CreatedAt: 1686935002,
				ID:        adapter.GetModel(openai.GPT4o),
				Object:    "model",
				OwnedBy:   owner,
			},
		},
	})
}

func ModelRetrieveHandler(c *gin.Context) {
	model := c.Param("model")
	owner := adapter.GetOwner()
	c.JSON(http.StatusOK, openai.Model{
		CreatedAt: 1686935002,
		ID:        model,
		Object:    "model",
		OwnedBy:   owner,
	})
}

func ChatProxyHandler(c *gin.Context) {
	// Retrieve the Authorization header value
	authorizationHeader := c.GetHeader("Authorization")
	// Declare a variable to store the OPENAI_API_KEY
	var openaiAPIKey string
	// Use fmt.Sscanf to extract the Bearer token
	_, err := fmt.Sscanf(authorizationHeader, "Bearer %s", &openaiAPIKey)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	req := &adapter.ChatCompletionRequest{}
	// Bind the JSON data from the request to the struct
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	req.Model = "Qwen/Qwen2.5-7B-Instruct"

	messages, err := req.ToGenaiMessages()
	if err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	config := openai.DefaultConfig(openaiAPIKey)
	config.BaseURL = "https://api.siliconflow.cn/v1"
	client := openai.NewClientWithConfig(config)

	model := req.ToGenaiModel()
	gemini := adapter.NewGeminiAdapter(client, model)

	if !req.Stream {
		resp, err := gemini.GenerateContent(ctx, req, messages)
		if err != nil {
			handleGenerateContentError(c, err)
			return
		}

		c.JSON(http.StatusOK, resp)
		return
	}

	dataChan, err := gemini.GenerateStreamContent(ctx, req, messages)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	setEventStreamHeaders(c)
	c.Stream(func(w io.Writer) bool {
		if data, ok := <-dataChan; ok {
			if data.Err != nil {
				byteData, _ := json.Marshal(data.Err)
				c.Render(-1, adapter.Event{Data: "data: " + string(byteData)})
			} else {
				byteData, _ := json.Marshal(data.Data)
				c.Render(-1, adapter.Event{Data: "data: " + string(byteData)})
			}
			return true
		}
		c.Render(-1, adapter.Event{Data: "data: [DONE]"})
		return false
	})
}

func handleGenerateContentError(c *gin.Context, err error) {
	log.Printf("genai generate content error %v\n", err)

	// Try OpenAI API error first
	var openaiErr *openai.APIError
	if errors.As(err, &openaiErr) {

		// Convert the code to an HTTP status code
		statusCode := http.StatusInternalServerError
		if code, ok := openaiErr.Code.(int); ok {
			statusCode = code
		}

		c.AbortWithStatusJSON(statusCode, openaiErr)
		return
	}

	// Try Google API error
	var googleErr *googleapi.Error
	if errors.As(err, &googleErr) {
		log.Printf("Handling Google API error with code: %d\n", googleErr.Code)
		statusCode := googleErr.Code
		if statusCode == http.StatusTooManyRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, openai.APIError{
				Code:    http.StatusTooManyRequests,
				Message: "Rate limit exceeded",
				Type:    "rate_limit_error",
			})
			return
		}

		c.AbortWithStatusJSON(statusCode, openai.APIError{
			Code:    statusCode,
			Message: googleErr.Message,
			Type:    "server_error",
		})
		return
	}

	// For all other errors
	log.Printf("Handling unknown error: %v\n", err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, openai.APIError{
		Code:    http.StatusInternalServerError,
		Message: err.Error(),
		Type:    "server_error",
	})
}

func setEventStreamHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
}

func EmbeddingProxyHandler(c *gin.Context) {
	// Retrieve the Authorization header value
	authorizationHeader := c.GetHeader("Authorization")
	// Declare a variable to store the OPENAI_API_KEY
	var openaiAPIKey string
	// Use fmt.Sscanf to extract the Bearer token
	_, err := fmt.Sscanf(authorizationHeader, "Bearer %s", &openaiAPIKey)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	req := &adapter.EmbeddingRequest{}
	// Bind the JSON data from the request to the struct
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	req.Model = adapter.TextEmbeddingBgeM3

	messages, err := req.ToGenaiMessages()
	if err != nil {
		c.JSON(http.StatusBadRequest, openai.APIError{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	config := openai.DefaultConfig(openaiAPIKey)
	config.BaseURL = "https://api.siliconflow.cn/v1"
	client := openai.NewClientWithConfig(config)

	model := req.ToGenaiModel()
	gemini := adapter.NewGeminiAdapter(client, model)

	resp, err := gemini.GenerateEmbedding(ctx, messages)
	if err != nil {
		handleGenerateContentError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
