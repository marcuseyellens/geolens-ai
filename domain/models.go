package domain

import "time"

// AIModelID 标识一个大语言模型。
type AIModelID string

const (
	ModelChatGPT   AIModelID = "chatgpt"
	ModelClaude    AIModelID = "claude"
	ModelGemini    AIModelID = "gemini"
	ModelPerplexity AIModelID = "perplexity"
)

// AllModels 返回平台支持的全部模型。
func AllModels() []AIModelID {
	return []AIModelID{ModelChatGPT, ModelClaude, ModelGemini, ModelPerplexity}
}

// Brand 是用户要监控的品牌或产品。
type Brand struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Website     string    `json:"website"`
	Description string    `json:"description"`
	Competitors []string  `json:"competitors"`
	Keywords    []string  `json:"keywords"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// QueryIntent 描述向 AI 模型提问的意图分类。
type QueryIntent string

const (
	IntentRecommend   QueryIntent = "recommend"
	IntentCompare     QueryIntent = "compare"
	IntentHowTo       QueryIntent = "howto"
	IntentBest        QueryIntent = "best"
	IntentAlternative QueryIntent = "alternative"
)

// DetectionQuery 是一条向 AI 模型发出的检测提问。
type DetectionQuery struct {
	ID        string      `json:"id"`
	BrandID   string      `json:"brand_id"`
	Text      string      `json:"text"`
	Intent    QueryIntent `json:"intent"`
	CreatedAt time.Time   `json:"created_at"`
}

// MentionPosition 是品牌在回答中被提及的位置。
type MentionPosition string

const (
	PositionFirst     MentionPosition = "first"
	PositionTop3      MentionPosition = "top3"
	PositionTop5      MentionPosition = "top5"
	PositionMentioned MentionPosition = "mentioned"
	PositionAbsent    MentionPosition = "absent"
)

// Sentiment 是提及的情感倾向。
type Sentiment string

const (
	SentimentPositive Sentiment = "positive"
	SentimentNeutral  Sentiment = "neutral"
	SentimentNegative Sentiment = "negative"
)

// DetectionResult 是一次 AI 模型检测的完整结果。
type DetectionResult struct {
	ID              string          `json:"id"`
	QueryID         string          `json:"query_id"`
	BrandID         string          `json:"brand_id"`
	Model           AIModelID       `json:"model"`
	ResponseText    string          `json:"response_text"`
	BrandMentioned  bool            `json:"brand_mentioned"`
	MentionPosition MentionPosition `json:"mention_position"`
	Sentiment       Sentiment       `json:"sentiment"`
	CompetitorsMentioned []string   `json:"competitors_mentioned"`
	Score           float64         `json:"score"`        // 0-100 综合得分
	ResponseTime    int             `json:"response_time_ms"`
	TokensUsed      int             `json:"tokens_used"`
	DetectedAt      time.Time       `json:"detected_at"`
}

// ReportSummary 是品牌在某个时间段内的检测汇总。
type ReportSummary struct {
	BrandID         string                  `json:"brand_id"`
	BrandName       string                  `json:"brand_name"`
	TotalDetections int                     `json:"total_detections"`
	MentionRate     float64                 `json:"mention_rate"`   // 被提及率 0-100
	AvgScore        float64                 `json:"avg_score"`      // 平均得分
	AvgPosition     float64                 `json:"avg_position"`   // 平均位置权重
	ModelBreakdown  map[AIModelID]ModelStat `json:"model_breakdown"`
	Trend           []TrendPoint            `json:"trend"`
}

// ModelStat 是单个模型的统计。
type ModelStat struct {
	Total       int     `json:"total"`
	Mentioned   int     `json:"mentioned"`
	MentionRate float64 `json:"mention_rate"`
	AvgScore    float64 `json:"avg_score"`
}

// TrendPoint 是趋势图上的一个点。
type TrendPoint struct {
	Date    string  `json:"date"`
	Score   float64 `json:"score"`
	MentionRate float64 `json:"mention_rate"`
}

// PositionWeight 将 MentionPosition 转换为数值权重。
func PositionWeight(p MentionPosition) float64 {
	switch p {
	case PositionFirst:
		return 100
	case PositionTop3:
		return 75
	case PositionTop5:
		return 50
	case PositionMentioned:
		return 25
	default:
		return 0
	}
}

// CalculateScore 根据位置和情感计算综合得分。
func CalculateScore(pos MentionPosition, sent Sentiment) float64 {
	base := PositionWeight(pos)
	if sent == SentimentNegative {
		base *= 0.5
	}
	if sent == SentimentNeutral {
		base *= 0.8
	}
	return base
}
