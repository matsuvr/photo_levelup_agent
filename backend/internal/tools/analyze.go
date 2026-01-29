package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

type AnalyzePhotoArgs struct {
	ImageURL string `json:"image_url" desc:"分析する画像のCloud Storage URL (gs://... 形式)"`
}

func analyzePhoto(tc tool.Context, args AnalyzePhotoArgs) (*services.AnalysisResult, error) {
	ctx := context.Background()
	geminiClient := services.NewGeminiClient()
	result, err := geminiClient.AnalyzeImage(ctx, args.ImageURL)
	if err != nil {
		return nil, fmt.Errorf("画像分析に失敗しました: %w", err)
	}

	resultJSON, _ := json.Marshal(result)
	if err := tc.State().Set("analysis_result", string(resultJSON)); err != nil {
		return nil, err
	}
	if err := tc.State().Set("original_image_url", args.ImageURL); err != nil {
		return nil, err
	}

	return result, nil
}

func NewAnalyzePhotoTool() (tool.Tool, error) {
	toolInstance, err := functiontool.New(
		functiontool.Config{
			Name: "analyze_photo",
			Description: "写真を詳細に分析し、構図、露出、色彩、ライティング、ピント、現像、距離感、意図の明確さ、の8項目を評価します。" +
				"各項目について10点満点でスコアリングし、改善点と総合コメントをJSONで返します。",
		},
		analyzePhoto,
	)
	if err != nil {
		return nil, err
	}
	return toolInstance, nil
}
