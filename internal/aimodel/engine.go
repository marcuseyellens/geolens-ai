package aimodel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/geolens/geolens/domain"
	"github.com/google/uuid"
)

// Engine 实现 application.AIEngine 接口
type Engine struct {
	apiKeys  map[domain.AIModelID]string
	client   *http.Client
	demoMode bool
}

// NewEngine 创建检测引擎，apiKeys 为各模型的 API Key。
// 未配置任何 Key（或显式设置 GEOLENS_DEMO=1）时自动进入演示模式，
// 用模拟回答跑通完整的分析、评分与报告链路，便于零配置体验产品。
func NewEngine(apiKeys map[domain.AIModelID]string) *Engine {
	configured := 0
	for _, key := range apiKeys {
		if key != "" {
			configured++
		}
	}
	return &Engine{
		apiKeys:  apiKeys,
		client:   &http.Client{Timeout: 60 * time.Second},
		demoMode: configured == 0 || os.Getenv("GEOLENS_DEMO") == "1",
	}
}

// IsDemo 返回当前是否处于演示模式
func (e *Engine) IsDemo() bool { return e.demoMode }

// Detect 使用单个模型检测
func (e *Engine) Detect(ctx context.Context, query *domain.DetectionQuery, brand *domain.Brand, model domain.AIModelID) (*domain.DetectionResult, error) {
	start := time.Now()
	respText, tokens, err := e.callModel(ctx, model, query.Text)
	if err != nil {
		return nil, fmt.Errorf("model %s: %w", model, err)
	}
	elapsed := time.Since(start)
	result := analyzeResponse(respText, brand, model)
	result.ID = uuid.New().String()
	result.QueryID = query.ID
	result.BrandID = brand.ID
	result.Model = model
	result.ResponseText = respText
	result.ResponseTime = int(elapsed.Milliseconds())
	result.TokensUsed = tokens
	result.DetectedAt = time.Now()
	return result, nil
}

// DetectAll 使用所有可用模型检测
func (e *Engine) DetectAll(ctx context.Context, query *domain.DetectionQuery, brand *domain.Brand) ([]*domain.DetectionResult, error) {
	var results []*domain.DetectionResult
	for _, model := range domain.AllModels() {
		if e.demoMode {
			results = append(results, e.demoDetect(query, brand, model))
			continue
		}
		if e.apiKeys[model] == "" {
			continue
		}
		result, err := e.Detect(ctx, query, brand, model)
		if err != nil {
			continue
		}
		results = append(results, result)
	}
	return results, nil
}

// demoDetect 生成一条演示检测结果，复用真实的回答分析逻辑
func (e *Engine) demoDetect(query *domain.DetectionQuery, brand *domain.Brand, model domain.AIModelID) *domain.DetectionResult {
	text := demoResponse(brand, query.Text, model)
	result := analyzeResponse(text, brand, model)
	result.ID = uuid.New().String()
	result.QueryID = query.ID
	result.BrandID = brand.ID
	result.ResponseText = text
	result.ResponseTime = 300 + len(text)%700
	result.TokensUsed = 150 + len(text)%200
	result.DetectedAt = time.Now()
	return result
}

// demoResponse 为不同模型构造有区分度的模拟回答，使报告能覆盖
// 首位/前3/未提及、正面/中性、竞品被提及等全部分支。
func demoResponse(brand *domain.Brand, question string, model domain.AIModelID) string {
	comp1, comp2 := "同类产品 A", "同类产品 B"
	if len(brand.Competitors) > 0 {
		comp1 = brand.Competitors[0]
	}
	if len(brand.Competitors) > 1 {
		comp2 = brand.Competitors[1]
	}
	name := brand.Name
	if name == "" {
		name = "该品牌"
	}
	q := strings.TrimSuffix(strings.TrimSpace(question), "?")
	if q == "" {
		q = "选择工具"
	}
	switch model {
	case domain.ModelChatGPT:
		return fmt.Sprintf(
			"%s is the best overall choice for %s. It delivers an excellent and reliable experience with a powerful, innovative feature set. "+
				"Most reviewers recommend it as a top solution for teams of any size. "+
				"%s and %s are also popular alternatives worth considering. "+
				"Overall, %s stands out for its quality and ease of use.",
			name, q, comp1, comp2, name)
	case domain.ModelClaude:
		return fmt.Sprintf(
			"When it comes to %s, %s is a leading option with a great reputation. "+
				"%s is another excellent tool that many teams rely on. "+
				"%s is a solid, reliable choice, though its setup can be complicated and its pricing is expensive for small teams. "+
				"I would compare all three before deciding.",
			q, comp1, comp2, name)
	case domain.ModelGemini:
		return fmt.Sprintf(
			"For %s, I would recommend %s and %s. Both are excellent, popular and widely used. "+
				"%s is especially powerful for larger organizations. "+
				"Consider your budget and workflow before making a final decision.",
			q, comp1, comp2, comp2)
	case domain.ModelPerplexity:
		return fmt.Sprintf(
			"Several tools are frequently cited for %s. %s is often rated as a top, excellent choice. "+
				"%s also appears in most comparisons as a reliable and innovative option. "+
				"%s is a great fit for users who need a powerful all-in-one workspace. "+
				"Many guides recommend %s as a leading solution.",
			q, comp1, comp2, name, name)
	default:
		return fmt.Sprintf("For %s, %s and %s are commonly mentioned.", q, comp1, comp2)
	}
}

// callModel 调用对应模型的 API，返回回答文本和 token 数
func (e *Engine) callModel(ctx context.Context, model domain.AIModelID, prompt string) (string, int, error) {
	switch model {
	case domain.ModelChatGPT:
		return e.callOpenAI(ctx, prompt)
	case domain.ModelClaude:
		return e.callAnthropic(ctx, prompt)
	case domain.ModelGemini:
		return e.callGemini(ctx, prompt)
	case domain.ModelPerplexity:
		return e.callPerplexity(ctx, prompt)
	default:
		return "", 0, fmt.Errorf("unsupported model: %s", model)
	}
}

func (e *Engine) callOpenAI(ctx context.Context, prompt string) (string, int, error) {
	key := e.apiKeys[domain.ModelChatGPT]
	body := map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	resp, err := e.doJSON(ctx, "https://api.openai.com/v1/chat/completions", key, body)
	if err != nil {
		return "", 0, err
	}
	text := resp["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"].(string)
	tokens := int(resp["usage"].(map[string]any)["total_tokens"].(float64))
	return text, tokens, nil
}

func (e *Engine) callAnthropic(ctx context.Context, prompt string) (string, int, error) {
	key := e.apiKeys[domain.ModelClaude]
	payload := map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 1024,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(data))
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("anthropic API error: %s", string(raw))
	}
	var result map[string]any
	json.Unmarshal(raw, &result)
	content := result["content"].([]any)[0].(map[string]any)["text"].(string)
	var tokens int
	if usage, ok := result["usage"].(map[string]any); ok {
		if ot, ok := usage["output_tokens"].(float64); ok {
			tokens = int(ot)
		}
	}
	return content, tokens, nil
}

func (e *Engine) callGemini(ctx context.Context, prompt string) (string, int, error) {
	key := e.apiKeys[domain.ModelGemini]
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", key)
	body := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
	}
	resp, err := e.doJSON(ctx, url, "", body)
	if err != nil {
		return "", 0, err
	}
	candidates := resp["candidates"].([]any)
	text := candidates[0].(map[string]any)["content"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
	var tokens int
	if usageMeta, ok := resp["usageMetadata"].(map[string]any); ok {
		if tc, ok := usageMeta["totalTokenCount"].(float64); ok {
			tokens = int(tc)
		}
	}
	return text, tokens, nil
}

func (e *Engine) callPerplexity(ctx context.Context, prompt string) (string, int, error) {
	key := e.apiKeys[domain.ModelPerplexity]
	body := map[string]any{
		"model":    "sonar",
		"messages": []map[string]string{{"role": "user", "content": prompt}},
	}
	resp, err := e.doJSON(ctx, "https://api.perplexity.ai/chat/completions", key, body)
	if err != nil {
		return "", 0, err
	}
	text := resp["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["content"].(string)
	tokens := int(resp["usage"].(map[string]any)["total_tokens"].(float64))
	return text, tokens, nil
}

func (e *Engine) doJSON(ctx context.Context, url, bearerKey string, body map[string]any) (map[string]any, error) {
	data, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if bearerKey != "" {
		req.Header.Set("Authorization", "Bearer "+bearerKey)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API %s returned %d: %s", url, resp.StatusCode, string(raw))
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}

// analyzeResponse 分析 AI 回答中品牌出现的位置和情感
func analyzeResponse(text string, brand *domain.Brand, model domain.AIModelID) *domain.DetectionResult {
	lowerText := strings.ToLower(text)
	brandName := strings.ToLower(brand.Name)

	// 检测品牌是否被提及
	brandMentioned := strings.Contains(lowerText, brandName)

	// 检测竞品提及
	var competitorsMentioned []string
	for _, comp := range brand.Competitors {
		if strings.Contains(lowerText, strings.ToLower(comp)) {
			competitorsMentioned = append(competitorsMentioned, comp)
		}
	}

	// 分析位置：按句子分割，找到品牌首次出现的位置
	position := domain.PositionAbsent
	if brandMentioned {
		sentences := splitSentences(text)
		for i, s := range sentences {
			if strings.Contains(strings.ToLower(s), brandName) {
				switch {
				case i == 0:
					position = domain.PositionFirst
				case i < 3:
					position = domain.PositionTop3
				case i < 5:
					position = domain.PositionTop5
				default:
					position = domain.PositionMentioned
				}
				break
			}
		}
		if position == domain.PositionAbsent {
			position = domain.PositionMentioned
		}
	}

	// 情感分析：简单关键词匹配
	sentiment := domain.SentimentNeutral
	if brandMentioned {
		brandContext := extractContext(text, brandName, 100)
		sentiment = detectSentiment(brandContext)
	}

	score := domain.CalculateScore(position, sentiment)

	return &domain.DetectionResult{
		BrandMentioned:       brandMentioned,
		MentionPosition:      position,
		Sentiment:            sentiment,
		CompetitorsMentioned: competitorsMentioned,
		Score:                score,
		Model:                model,
	}
}

func splitSentences(text string) []string {
	text = strings.ReplaceAll(text, "\n", ". ")
	text = strings.ReplaceAll(text, "!", ". ")
	text = strings.ReplaceAll(text, "?", ". ")
	parts := strings.Split(text, ". ")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func extractContext(text, target string, window int) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, target)
	if idx == -1 {
		return text
	}
	start := idx - window
	if start < 0 {
		start = 0
	}
	end := idx + len(target) + window
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}

func detectSentiment(text string) domain.Sentiment {
	lower := strings.ToLower(text)
	positiveWords := []string{"best", "excellent", "great", "recommend", "top", "leading", "powerful", "innovative", "popular", "reliable", "quality", "superior"}
	negativeWords := []string{"poor", "bad", "terrible", "avoid", "disappointing", "limited", "expensive", "complicated", "outdated", "inferior", "issue", "problem"}

	posCount := 0
	negCount := 0
	for _, w := range positiveWords {
		if strings.Contains(lower, w) {
			posCount++
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(lower, w) {
			negCount++
		}
	}
	switch {
	case posCount > negCount:
		return domain.SentimentPositive
	case negCount > posCount:
		return domain.SentimentNegative
	default:
		return domain.SentimentNeutral
	}
}
