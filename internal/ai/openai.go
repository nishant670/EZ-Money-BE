package ai

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"finance-parser-go/internal/config"
)

//go:embed prompt.txt
var promptText string

type OpenAIClient struct {
	cfg  *config.Config
	http *http.Client
}

func NewOpenAIClient(cfg *config.Config) *OpenAIClient {
	return &OpenAIClient{cfg: cfg, http: &http.Client{}}
}

func (c *OpenAIClient) Transcribe(ctx context.Context, filename string, audio []byte) (string, error) {
	if c.cfg.OpenAIKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY missing")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", filename)
	if _, err := io.Copy(fw, bytes.NewReader(audio)); err != nil {
		return "", err
	}
	_ = mw.WriteField("model", c.cfg.OpenAIWhisper)
	_ = mw.Close()

	req, _ := http.NewRequestWithContext(ctx, "POST", c.cfg.OpenAIBaseURL+"/audio/transcriptions", &buf)
	req.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper error: %s", string(b))
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Text), nil
}

func (c *OpenAIClient) ParseText(ctx context.Context, transcript, tz string) ([]byte, error) {
	if c.cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY missing")
	}

	// Calculate current date in User's TZ
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc, _ = time.LoadLocation("Asia/Kolkata")
	}
	if loc == nil {
		loc = time.FixedZone("IST", 5*3600+1800)
	}
	nowStr := time.Now().In(loc).Format("2006-01-02")

	body := map[string]any{
		"model":           c.cfg.OpenAILlmModel,
		"response_format": map[string]string{"type": "json_object"},
		"messages": []map[string]string{
			{"role": "system", "content": promptText},
			{"role": "user", "content": fmt.Sprintf("Context: Timezone is %s. Today is %s.\nText: %s", tz, nowStr, transcript)},
		},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.cfg.OpenAIBaseURL+"/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.cfg.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		bs, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm error: %s", string(bs))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, fmt.Errorf("no choices")
	}
	return []byte(out.Choices[0].Message.Content), nil
}
