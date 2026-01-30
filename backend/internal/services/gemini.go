package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

type GeminiClient struct {
	client *genai.Client
}

type AnalysisResult struct {
	Summary        string        `json:"summary"`
	OverallComment string        `json:"overallComment"`
	OverallScore   int           `json:"overallScore"`
	Composition    CategoryScore `json:"composition"`
	Exposure       CategoryScore `json:"exposure"`
	Color          CategoryScore `json:"color"`
	Lighting       CategoryScore `json:"lighting"`
	Focus          CategoryScore `json:"focus"`
	Development    CategoryScore `json:"development"`
	Distance       CategoryScore `json:"distance"`
	IntentClarity  CategoryScore `json:"intentClarity"`
}

type CategoryScore struct {
	Score       int    `json:"score"`
	Comment     string `json:"comment"`
	Improvement string `json:"improvement"`
}

type EnhancementInput struct {
	ImageURL    string
	Analysis    *AnalysisResult
	CustomNotes string
}

type ImageGenerationResult struct {
	ImageBase64 string
	Reasoning   string
}

func NewGeminiClient() *GeminiClient {
	return &GeminiClient{}
}

func (g *GeminiClient) Ensure(ctx context.Context) error {
	if g.client != nil {
		return nil
	}

	if err := os.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true"); err != nil {
		return err
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  os.Getenv("GOOGLE_CLOUD_PROJECT"),
		Location: resolveLocation(),
		Backend:  genai.BackendVertexAI,
		HTTPOptions: genai.HTTPOptions{
			APIVersion: "v1",
		},
	})
	if err != nil {
		return err
	}

	g.client = client
	return nil
}

func (g *GeminiClient) AnalyzeImage(ctx context.Context, imageURL string) (*AnalysisResult, error) {
	if err := g.Ensure(ctx); err != nil {
		return nil, err
	}

	analysisPrompt := strings.Join([]string{
		"あなたは写真講評のプロです。次の写真を詳細に評価してください。",
		"採点項目は構図、露出、色彩、ライティング、ピント、現像、距離感、意図の明確さの8項目です。",
		"各項目は0〜10点で採点し、短い講評コメントと具体的な改善提案を必ず記述してください。",
		"全体サマリーと総合コメント、平均点(0〜10)も作成してください。",
		"出力は日本語で、指定されたJSONスキーマに厳密に従ってください。",
	}, "\n")
	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(analysisPrompt),
			genai.NewPartFromURI(imageURL, "image/jpeg"),
		}, genai.RoleUser),
	}

	response, err := g.client.Models.GenerateContent(ctx, modelName(), contents, &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   analysisResponseSchema(),
		Tools: []*genai.Tool{
			{CodeExecution: &genai.ToolCodeExecution{}},
		},
	})
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(response.Text())
	if text == "" {
		return nil, errors.New("empty analysis response")
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("analysis response parse failed: %w", err)
	}
	return &result, nil
}

func (g *GeminiClient) CompareAndAdvise(ctx context.Context, originalURL, transformedURL, analysisJSON string) (string, error) {
	if err := g.Ensure(ctx); err != nil {
		return "", err
	}

	analysisText := analysisJSON
	if analysisJSON != "" {
		var parsed AnalysisResult
		if err := json.Unmarshal([]byte(analysisJSON), &parsed); err == nil && parsed.Summary != "" {
			analysisText = parsed.Summary
		}
	}

	prompt := fmt.Sprintf("元の写真と改善案の写真を比較し、改善点とアドバイスを具体的に説明してください。\n分析結果: %s", analysisText)
	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(prompt),
			genai.NewPartFromURI(originalURL, "image/jpeg"),
			genai.NewPartFromURI(transformedURL, "image/jpeg"),
		}, genai.RoleUser),
	}

	response, err := g.client.Models.GenerateContent(ctx, modelName(), contents, &genai.GenerateContentConfig{})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response.Text()), nil
}

func (g *GeminiClient) GenerateImage(ctx context.Context, prompt string) (*ImageGenerationResult, error) {
	if err := g.Ensure(ctx); err != nil {
		return nil, err
	}

	config := &genai.GenerateContentConfig{ResponseModalities: []string{"IMAGE", "TEXT"}}
	response, err := g.client.Models.GenerateContent(ctx, "gemini-3-pro-image-preview", genai.Text(prompt), config)
	if err != nil {
		return nil, err
	}

	if len(response.Candidates) == 0 || response.Candidates[0].Content == nil {
		return nil, errors.New("empty image generation response")
	}

	var imageBase64 string
	var reasoning string
	for _, part := range response.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			imageBase64 = base64.StdEncoding.EncodeToString(part.InlineData.Data)
		}
		if part.Text != "" {
			reasoning += part.Text
		}
	}

	if imageBase64 == "" {
		return nil, errors.New("image data missing in response")
	}

	return &ImageGenerationResult{
		ImageBase64: imageBase64,
		Reasoning:   strings.TrimSpace(reasoning),
	}, nil
}

func (g *GeminiClient) EnhancePhoto(ctx context.Context, input EnhancementInput) (*ImageGenerationResult, error) {
	if err := g.Ensure(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ImageURL) == "" {
		return nil, errors.New("image url is required")
	}

	prompt := buildEnhancementPrompt(input)
	config := &genai.GenerateContentConfig{ResponseModalities: []string{"IMAGE", "TEXT"}}
	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(prompt),
			genai.NewPartFromURI(input.ImageURL, "image/jpeg"),
		}, genai.RoleUser),
	}
	response, err := g.client.Models.GenerateContent(ctx, "gemini-3-pro-image-preview", contents, config)
	if err != nil {
		return nil, err
	}

	if len(response.Candidates) == 0 || response.Candidates[0].Content == nil {
		return nil, errors.New("empty image generation response")
	}

	var imageBase64 string
	var reasoning string
	for _, part := range response.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			imageBase64 = base64.StdEncoding.EncodeToString(part.InlineData.Data)
		}
		if part.Text != "" {
			reasoning += part.Text
		}
	}

	if imageBase64 == "" {
		return nil, errors.New("image data missing in response")
	}

	return &ImageGenerationResult{
		ImageBase64: imageBase64,
		Reasoning:   strings.TrimSpace(reasoning),
	}, nil
}

func buildEnhancementPrompt(input EnhancementInput) string {
	analysisDetails := formatEnhancementAnalysis(input.Analysis)
	if analysisDetails == "" {
		analysisDetails = "構図・露出・色彩・ライティングをより洗練されたコンテスト受賞レベルに高めてください。"
	}

	customNotes := strings.TrimSpace(input.CustomNotes)
	if customNotes != "" {
		customNotes = "\n追加の要望: " + customNotes
	}

	return fmt.Sprintf("あなたはプロの写真レタッチャー兼講師です。元写真の内容は維持したまま、自然で高品質な改善を行い、コンテスト受賞レベルの仕上がりにしてください。\n\n採点結果と改善提案: %s%s\n\n重要: 生成される画像は「改善後の美しい写真」ですが、そこに「赤ペン先生」のように、改善ポイントや良くなった部分に赤丸や矢印をつけ、手書き風の文字で短いコメント（例:「ここを明るく」「構図を整理」など）を書き込んでください。改善された写真そのものに、直接赤ペンで書き込みが入っている状態の画像を出力してください。\n\n改善ルール:\n1. 被写道の基礎は維持しつつ、プロレベルに仕上げる\n2. 改善ポイントに赤ペンでマルや矢印を入れる\n3. 手書き風の文字でコメントを入れる\n4. 露出、色彩、ライティングを最適化\n\n変換後の写真（赤ペン添削付き）を生成し、変更点を簡潔に説明してください。", analysisDetails, customNotes)
}

func formatEnhancementAnalysis(analysis *AnalysisResult) string {
	if analysis == nil {
		return ""
	}

	parts := []string{}
	if summary := strings.TrimSpace(analysis.Summary); summary != "" {
		parts = append(parts, fmt.Sprintf("サマリー: %s", summary))
	}
	if overall := strings.TrimSpace(analysis.OverallComment); overall != "" {
		parts = append(parts, fmt.Sprintf("総合コメント: %s", overall))
	}
	if analysis.OverallScore > 0 {
		parts = append(parts, fmt.Sprintf("総合スコア: %d/10", analysis.OverallScore))
	}

	categoryLines := buildCategoryLines([]categorySummary{
		{name: "構図", score: analysis.Composition.Score, comment: analysis.Composition.Comment, improvement: analysis.Composition.Improvement},
		{name: "露出", score: analysis.Exposure.Score, comment: analysis.Exposure.Comment, improvement: analysis.Exposure.Improvement},
		{name: "色彩", score: analysis.Color.Score, comment: analysis.Color.Comment, improvement: analysis.Color.Improvement},
		{name: "ライティング", score: analysis.Lighting.Score, comment: analysis.Lighting.Comment, improvement: analysis.Lighting.Improvement},
		{name: "ピント", score: analysis.Focus.Score, comment: analysis.Focus.Comment, improvement: analysis.Focus.Improvement},
		{name: "現像", score: analysis.Development.Score, comment: analysis.Development.Comment, improvement: analysis.Development.Improvement},
		{name: "距離感", score: analysis.Distance.Score, comment: analysis.Distance.Comment, improvement: analysis.Distance.Improvement},
		{name: "意図の明確さ", score: analysis.IntentClarity.Score, comment: analysis.IntentClarity.Comment, improvement: analysis.IntentClarity.Improvement},
	})
	if len(categoryLines) > 0 {
		parts = append(parts, "項目別の改善提案:\n"+strings.Join(categoryLines, "\n"))
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

type categorySummary struct {
	name        string
	score       int
	comment     string
	improvement string
}

func buildCategoryLines(categories []categorySummary) []string {
	lines := make([]string, 0, len(categories))
	for _, category := range categories {
		comment := strings.TrimSpace(category.comment)
		improvement := strings.TrimSpace(category.improvement)
		lineParts := []string{fmt.Sprintf("- %s: %d/10", category.name, category.score)}
		if comment != "" {
			lineParts = append(lineParts, fmt.Sprintf("講評: %s", comment))
		}
		if improvement != "" {
			lineParts = append(lineParts, fmt.Sprintf("改善提案: %s", improvement))
		}
		lines = append(lines, strings.Join(lineParts, " / "))
	}
	return lines
}

func resolveLocation() string {
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if location == "" {
		location = os.Getenv("GOOGLE_CLOUD_REGION")
	}
	return location
}

func modelName() string {
	if name := os.Getenv("VERTEXAI_LLM"); name != "" {
		return name
	}
	return "gemini-3-pro-preview"
}

func analysisResponseSchema() *genai.Schema {
	minScore := float64(0)
	maxScore := float64(10)
	categorySchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type:        genai.TypeInteger,
				Minimum:     &minScore,
				Maximum:     &maxScore,
				Description: "0から10の整数で評価する",
			},
			"comment": {
				Type:        genai.TypeString,
				Description: "現状に対する講評コメント",
			},
			"improvement": {
				Type:        genai.TypeString,
				Description: "具体的な改善提案",
			},
		},
		Required: []string{"score", "comment", "improvement"},
	}

	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"summary": {
				Type:        genai.TypeString,
				Description: "全体サマリー",
			},
			"overallComment": {
				Type:        genai.TypeString,
				Description: "総合的なコメント",
			},
			"overallScore": {
				Type:        genai.TypeInteger,
				Minimum:     &minScore,
				Maximum:     &maxScore,
				Description: "8項目の平均点(0-10の整数)",
			},
			"composition":   categorySchema,
			"exposure":      categorySchema,
			"color":         categorySchema,
			"lighting":      categorySchema,
			"focus":         categorySchema,
			"development":   categorySchema,
			"distance":      categorySchema,
			"intentClarity": categorySchema,
		},
		Required: []string{
			"summary",
			"overallComment",
			"overallScore",
			"composition",
			"exposure",
			"color",
			"lighting",
			"focus",
			"development",
			"distance",
			"intentClarity",
		},
		PropertyOrdering: []string{
			"summary",
			"overallComment",
			"overallScore",
			"composition",
			"exposure",
			"color",
			"lighting",
			"focus",
			"development",
			"distance",
			"intentClarity",
		},
	}
}
