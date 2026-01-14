package scorer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"info-filter/internal/models"
)

type Scorer struct {
	apiKey   string
	model    string
	endpoint string
	client   *http.Client
}

func NewScorer() *Scorer {
	return &Scorer{
		apiKey:   os.Getenv("GEMINI_API_KEY"),
		model:    getEnvOrDefault("GEMINI_MODEL", "gemini-2.0-flash"),
		endpoint: getEnvOrDefault("GEMINI_API_ENDPOINT", "https://generativelanguage.googleapis.com"),
		client:   &http.Client{Timeout: 60 * time.Second},
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

const scorePrompt = `你是一个信息价值评估专家。请对以下信息进行四维度评分。

## 评分标准

1. **稀缺性** (0-25分): 这个信息有多少人已经知道？
   - 首发/早期传播 = 20-25分
   - 小范围传播 = 10-19分
   - 已经热门 = 0-9分

2. **可操作性** (0-25分): 24小时内能产生什么行动？
   - 能立刻采取具体行动 = 20-25分
   - 需要一些准备才能行动 = 10-19分
   - 只是"知道了"，无法行动 = 0-9分

3. **杠杆率** (0-25分): 投入产出比如何？
   - 小投入大回报潜力 = 20-25分
   - 中等投入中等回报 = 10-19分
   - 大投入小回报 = 0-9分

4. **人性共鸣** (0-25分): 基于卡耐基《人性的弱点》，细分为三个子维度：
   - **重要感** (0-8分): 分享这个能让人显得牛逼/领先吗？
   - **利益相关** (0-9分): 能帮人省钱/赚钱/省时间吗？
   - **高尚动机** (0-8分): 有改变世界/造福他人的故事感吗？

## 待评估信息

标题: %s
来源: %s
描述: %s

## 输出格式

请严格按以下JSON格式输出，不要有其他内容：
{"scarcity": <0-25>, "actionable": <0-25>, "leverage": <0-25>, "importance": <0-8>, "benefit": <0-9>, "noble": <0-8>, "reason": "<一句话说明为什么这个分数>"}`

// Gemini API 请求结构
type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

// Gemini API 响应结构
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *Scorer) Score(ctx context.Context, item models.Item) (models.ScoreResult, error) {
	prompt := fmt.Sprintf(scorePrompt, item.Title, item.Source, item.Description)

	// 构建请求
	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{Parts: []GeminiPart{{Text: prompt}}},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return models.ScoreResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 调用 Gemini API（支持自定义代理）
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.endpoint, s.model, s.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return models.ScoreResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return models.ScoreResult{}, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.ScoreResult{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return models.ScoreResult{}, fmt.Errorf("API error: %s", string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return models.ScoreResult{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return models.ScoreResult{}, fmt.Errorf("Gemini error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return models.ScoreResult{}, fmt.Errorf("empty response from Gemini")
	}

	content := geminiResp.Candidates[0].Content.Parts[0].Text

	// 提取JSON（可能包含markdown代码块）
	content = extractJSON(content)

	var result models.ScoreResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return models.ScoreResult{}, fmt.Errorf("failed to parse score JSON: %w, content: %s", err, content)
	}

	// 计算人性共鸣总分和总分
	result.Resonance = result.Importance + result.Benefit + result.Noble
	result.Total = result.Scarcity + result.Actionable + result.Leverage + result.Resonance

	return result, nil
}

// 从可能包含markdown代码块的文本中提取JSON
func extractJSON(text string) string {
	// 尝试找到 ```json ... ``` 块
	start := 0
	if idx := indexOf(text, "```json"); idx != -1 {
		start = idx + 7
	} else if idx := indexOf(text, "```"); idx != -1 {
		start = idx + 3
	}

	end := len(text)
	if idx := lastIndexOf(text, "```"); idx > start {
		end = idx
	}

	result := text[start:end]
	// 去掉首尾空白
	for len(result) > 0 && (result[0] == '\n' || result[0] == ' ') {
		result = result[1:]
	}
	for len(result) > 0 && (result[len(result)-1] == '\n' || result[len(result)-1] == ' ') {
		result = result[:len(result)-1]
	}

	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func lastIndexOf(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

const askPrompt = `你是一个信息分析助手。用户对以下信息有疑问，请基于信息内容进行详细解答。

## 信息内容

标题: %s
来源: %s
描述: %s
链接: %s

## 用户问题

%s

## 要求

1. 基于提供的信息内容进行解答
2. 如果信息不足以回答，请诚实说明
3. 用简洁清晰的中文回答
4. 如果能提供更多背景知识会更好`

// Ask 回答用户关于某条信息的问题
func (s *Scorer) Ask(ctx context.Context, item models.Item, question string) (string, error) {
	prompt := fmt.Sprintf(askPrompt, item.Title, item.Source, item.Description, item.URL, question)

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{Parts: []GeminiPart{{Text: prompt}}},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", s.endpoint, s.model, s.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s", string(body))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("Gemini error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}
