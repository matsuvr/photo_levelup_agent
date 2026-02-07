package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/genai"
)

type GeminiClient struct {
	client *genai.Client
}

type AnalysisResult struct {
	PhotoSummary   string        `json:"photoSummary"`
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

	apiKey := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
	if apiKey == "" {
		return errors.New("GOOGLE_API_KEY is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return err
	}

	g.client = client
	return nil
}

// fetchImageBytes fetches image data from GCS URL or HTTP URL and returns the bytes
func fetchImageBytes(ctx context.Context, imageURL string) ([]byte, string, error) {
	if strings.HasPrefix(imageURL, "gs://") {
		return fetchFromGCS(ctx, imageURL)
	}
	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		return fetchFromHTTP(ctx, imageURL)
	}
	return nil, "", fmt.Errorf("unsupported URL format: %s", imageURL)
}

// fetchFromGCS fetches image data directly from Google Cloud Storage
func fetchFromGCS(ctx context.Context, gcsURL string) ([]byte, string, error) {
	trimmed := strings.TrimPrefix(gcsURL, "gs://")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid GCS URL format: %s", gcsURL)
	}

	bucketName := parts[0]
	objectName := parts[1]

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	reader, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read GCS object: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	// Detect MIME type from content type or default to image/jpeg
	mimeType := reader.Attrs.ContentType
	if mimeType == "" {
		mimeType = detectMimeType(objectName)
	}

	log.Printf("DEBUG: Fetched %d bytes from GCS: %s (mime: %s)", len(data), gcsURL, mimeType)
	return data, mimeType, nil
}

// fetchFromHTTP fetches image data from HTTP/HTTPS URL
func fetchFromHTTP(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch image: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = detectMimeType(url)
	}

	log.Printf("DEBUG: Fetched %d bytes from HTTP: %s (mime: %s)", len(data), url, mimeType)
	return data, mimeType, nil
}

// detectMimeType detects MIME type from file extension
func detectMimeType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func (g *GeminiClient) AnalyzeImage(ctx context.Context, imageURL string) (*AnalysisResult, error) {
	log.Printf("DEBUG: AnalyzeImage called with URL: %s", imageURL)
	if err := g.Ensure(ctx); err != nil {
		return nil, err
	}

	// Fetch image data directly instead of using signed URL
	imageData, mimeType, err := fetchImageBytes(ctx, imageURL)
	if err != nil {
		log.Printf("ERROR: Failed to fetch image data: %v", err)
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	log.Printf("DEBUG: Fetched image data (%d bytes, %s) for analysis", len(imageData), mimeType)

	analysisPrompt := strings.Join([]string{
		"あなたは写真講評のプロです。次の写真を詳細に評価してください。",
		"採点項目は構図、露出、色彩、ライティング、ピント、現像、距離感、意図の明確さの8項目です。",
		"各項目は0〜10点で採点し、短い講評コメントと具体的な改善提案を必ず記述してください。",
		"また、写真の内容を一言でまとめたタイトル(photoSummary)を作成してください。",
		"全体サマリーと総合コメント、平均点(0〜10)も作成してください。",
		"出力は日本語で、指定されたJSONスキーマに厳密に従ってください。",
	}, "\n")
	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(analysisPrompt),
			genai.NewPartFromBytes(imageData, mimeType),
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
		log.Printf("ERROR: AnalyzeImage GenerateContent failed: %v", err)
		return nil, err
	}

	text := strings.TrimSpace(response.Text())
	if text == "" {
		log.Printf("ERROR: AnalyzeImage returned empty response")
		return nil, errors.New("empty analysis response")
	}

	var result AnalysisResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		log.Printf("ERROR: AnalyzeImage JSON unmarshal failed: %v", err)
		return nil, fmt.Errorf("analysis response parse failed: %w", err)
	}
	return &result, nil
}

func (g *GeminiClient) CompareAndAdvise(ctx context.Context, originalURL, transformedURL, analysisJSON string) (string, error) {
	if err := g.Ensure(ctx); err != nil {
		return "", err
	}

	// Fetch image data directly instead of using signed URLs
	originalData, originalMime, err := fetchImageBytes(ctx, originalURL)
	if err != nil {
		log.Printf("ERROR: Failed to fetch original image: %v", err)
		return "", fmt.Errorf("failed to fetch original image: %w", err)
	}
	transformedData, transformedMime, err := fetchImageBytes(ctx, transformedURL)
	if err != nil {
		log.Printf("ERROR: Failed to fetch transformed image: %v", err)
		return "", fmt.Errorf("failed to fetch transformed image: %w", err)
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
			genai.NewPartFromBytes(originalData, originalMime),
			genai.NewPartFromBytes(transformedData, transformedMime),
		}, genai.RoleUser),
	}

	response, err := g.client.Models.GenerateContent(ctx, modelName(), contents, &genai.GenerateContentConfig{})
	if err != nil {
		return "", err
	}

	return fixMarkdownBold(strings.TrimSpace(response.Text())), nil
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
	return g.enhancePhotoWithPrompt(ctx, input, buildEnhancementPrompt)
}

func (g *GeminiClient) EnhancePhotoClean(ctx context.Context, input EnhancementInput) (*ImageGenerationResult, error) {
	return g.enhancePhotoWithPrompt(ctx, input, buildCleanEnhancementPrompt)
}

func (g *GeminiClient) enhancePhotoWithPrompt(ctx context.Context, input EnhancementInput, promptBuilder func(EnhancementInput) string) (*ImageGenerationResult, error) {
	if err := g.Ensure(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ImageURL) == "" {
		return nil, errors.New("image url is required")
	}

	// Fetch image data directly instead of using signed URL
	imageData, mimeType, err := fetchImageBytes(ctx, input.ImageURL)
	if err != nil {
		log.Printf("ERROR: Failed to fetch image in EnhancePhoto: %v", err)
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}

	prompt := promptBuilder(input)
	config := &genai.GenerateContentConfig{ResponseModalities: []string{"IMAGE", "TEXT"}}
	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(prompt),
			genai.NewPartFromBytes(imageData, mimeType),
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

func buildCleanEnhancementPrompt(input EnhancementInput) string {
	analysisDetails := formatEnhancementAnalysis(input.Analysis)
	if analysisDetails == "" {
		analysisDetails = "構図・露出・色彩・ライティングをより洗練されたコンテスト受賞レベルに高めてください。"
	}

	customNotes := strings.TrimSpace(input.CustomNotes)
	if customNotes != "" {
		customNotes = "\n追加の要望: " + customNotes
	}

	return fmt.Sprintf("あなたはプロの写真レタッチャー兼講師です。元写真の内容は維持したまま、自然で高品質な改善を行い、コンテスト受賞レベルの仕上がりにしてください。\n\n採点結果と改善提案: %s%s\n\n重要: 注釈・文字・矢印・赤ペンなどの書き込みは一切行わないでください。改善された美しい写真のみを出力してください。クリーンで美しい仕上がりの写真だけを生成してください。\n\n改善ルール:\n1. 被写体の基礎は維持しつつ、プロレベルに仕上げる\n2. 露出、色彩、ライティングを最適化\n3. 構図の改善を反映\n4. 文字や注釈は一切入れない\n\n変換後の写真（クリーンな改善版）を生成し、変更点を簡潔に説明してください。", analysisDetails, customNotes)
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

func modelName() string {
	if name := os.Getenv("GEMINI_MODEL"); name != "" {
		return name
	}
	return "gemini-3-flash-preview"
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
			"photoSummary": {
				Type:        genai.TypeString,
				Description: "写真の内容を一言でまとめたタイトル",
			},
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
			"photoSummary",
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
			"photoSummary",
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

// fixMarkdownBold fixes markdown bold syntax by removing spaces between ** and text.
// Examples:
//   - "** text **" -> "**text**"
//   - "** text * text * text **" -> "**text * text * text**"
//   - "**text**" -> "**text**" (no change)
func fixMarkdownBold(text string) string {
	// Pattern: \*\* matches **, \s+ matches one or more spaces, (.+?) captures content (non-greedy), \s+ matches spaces, \*\* matches **
	// We need to handle the case where there might be single * inside double **
	re := regexp.MustCompile(`\*\*\s+(.+?)\s+\*\*`)
	return re.ReplaceAllString(text, `**$1**`)
}
