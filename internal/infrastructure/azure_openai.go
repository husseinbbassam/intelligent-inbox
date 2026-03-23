package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
)

// azureOpenAIJudge implements domain.AIJudge using Azure OpenAI Chat Completions.
type azureOpenAIJudge struct {
	client     *azopenai.Client
	deployment string // Azure OpenAI deployment name (model alias)
}

// NewAzureOpenAIJudge constructs an AIJudge backed by Azure OpenAI.
// endpoint and apiKey are taken from the environment.
func NewAzureOpenAIJudge(endpoint, apiKey, deployment string) (domain.AIJudge, error) {
	keyCredential := azcore.NewKeyCredential(apiKey)
	client, err := azopenai.NewClientWithKeyCredential(endpoint, keyCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("create azure openai client: %w", err)
	}
	return &azureOpenAIJudge{client: client, deployment: deployment}, nil
}

// Judge evaluates a raw JSON payload and returns a JudgeResult.
// It injects Few-Shot human feedback examples into the prompt for self-learning.
func (j *azureOpenAIJudge) Judge(
	ctx context.Context,
	systemID domain.SystemID,
	rawData []byte,
	fewShots []*domain.HumanFeedback,
) (*domain.JudgeResult, error) {
	prompt := buildPrompt(systemID, rawData, fewShots)

	messages := []azopenai.ChatRequestMessageClassification{
		&azopenai.ChatRequestSystemMessage{
			Content: azopenai.NewChatRequestSystemMessageContent(systemPrompt()),
		},
		&azopenai.ChatRequestUserMessage{
			Content: azopenai.NewChatRequestUserMessageContent(prompt),
		},
	}

	resp, err := j.client.GetChatCompletions(ctx, azopenai.ChatCompletionsOptions{
		DeploymentName: &j.deployment,
		Messages:       messages,
		MaxTokens:      toPtr(int32(512)),
		Temperature:    toPtr(float32(0.0)),
	}, nil)

	if err != nil {
		return nil, classifyAzureError(err)
	}

	if len(resp.Choices) == 0 {
		return nil, errors.New("azure openai returned no choices")
	}

	content := ""
	if resp.Choices[0].Message != nil && resp.Choices[0].Message.Content != nil {
		content = *resp.Choices[0].Message.Content
	}

	result, err := parseJudgeResponse(content)
	if err != nil {
		log.Printf("WARN: failed to parse AI response: %v — raw: %s", err, content)
		return nil, fmt.Errorf("parse judge response: %w", err)
	}

	return result, nil
}

// systemPrompt returns the fixed system-level instruction for the AI.
func systemPrompt() string {
	return `You are a Data Quality Judge for an intelligent data ingestion system.
Your job is to analyze incoming data records and classify them as either CLEAN or NOISE.

NOISE examples:
- VendorName that is gibberish (e.g. "asdfghj123", "xxxx", "!!!!")
- Invoice TotalAmount that is negative (e.g. -500)
- Description containing only special characters or whitespace
- DeviceID that is empty or a random string with no pattern
- Values that are physically impossible for the sensor type

Respond ONLY in the following JSON format (no markdown, no extra text):
{
  "score": <float between 0.0 and 1.0, where 1.0 = perfectly clean>,
  "status": "<CLEAN|NOISE>",
  "reasoning": "<one sentence explanation>"
}`
}

// buildPrompt constructs the user-turn prompt, including Few-Shot examples.
func buildPrompt(systemID domain.SystemID, rawData []byte, fewShots []*domain.HumanFeedback) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## System: %s\n\n", systemID))

	if len(fewShots) > 0 {
		sb.WriteString("## Context (learned from human corrections)\n")
		sb.WriteString("The following examples show previous corrections made by human reviewers.\n")
		sb.WriteString("Use them to calibrate your judgment:\n\n")
		for i, fs := range fewShots {
			sb.WriteString(fmt.Sprintf("### Example %d\n", i+1))
			sb.WriteString(fmt.Sprintf("Data: %s\n", string(fs.OriginalData)))
			sb.WriteString(fmt.Sprintf("Human Label: %s\n", fs.CorrectLabel))
			if fs.CorrectionNote != "" {
				sb.WriteString(fmt.Sprintf("Reason: %s\n", fs.CorrectionNote))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("## Record to Evaluate\n")
	sb.WriteString(string(rawData))
	sb.WriteString("\n\nClassify the above record and respond in the required JSON format.")

	return sb.String()
}

// parseJudgeResponse attempts to decode the LLM's JSON response into a JudgeResult.
func parseJudgeResponse(content string) (*domain.JudgeResult, error) {
	// Strip potential markdown code fences if the model ignores instructions.
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var raw struct {
		Score     float64 `json:"score"`
		Status    string  `json:"status"`
		Reasoning string  `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("unmarshal judge json: %w", err)
	}

	status := domain.AIStatus(strings.ToUpper(raw.Status))
	if status != domain.AIStatusClean && status != domain.AIStatusNoise {
		return nil, fmt.Errorf("unknown status value: %q", raw.Status)
	}

	return &domain.JudgeResult{
		Score:     clamp(raw.Score, 0, 1),
		Status:    status,
		Reasoning: raw.Reasoning,
	}, nil
}

// classifyAzureError maps transient Azure HTTP errors to a sentinel so callers
// can decide whether to retry (PENDING_RETRY) or surface the error.
func classifyAzureError(err error) error {
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.StatusCode {
		case http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError:
			return &TransientError{Cause: err, StatusCode: respErr.StatusCode}
		}
	}
	return err
}

// TransientError signals that the upstream LLM call failed in a retryable way.
type TransientError struct {
	Cause      error
	StatusCode int
}

func (e *TransientError) Error() string {
	return fmt.Sprintf("transient azure openai error (HTTP %d): %v", e.StatusCode, e.Cause)
}

func (e *TransientError) Unwrap() error { return e.Cause }

// helpers

func toPtr[T any](v T) *T { return &v }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
