package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

type AnalyzePhotoArgs struct {
	ImageURL string `json:"image_url" desc:"分析する画像のCloud Storage URL (gs://... 形式)"`
}

func analyzePhoto(tc tool.Context, args AnalyzePhotoArgs) (*services.AnalysisResult, error) {
	log.Printf("DEBUG: analyzePhoto tool called with args: %+v", args)
	ctx := context.Background()
	geminiClient := services.NewGeminiClient()
	result, err := geminiClient.AnalyzeImage(ctx, args.ImageURL)
	if err != nil {
		log.Printf("ERROR: analyzePhoto tool failed: %v", err)
		return nil, fmt.Errorf("画像分析に失敗しました: %w", err)
	}

	resultJSON, _ := json.Marshal(result)
	if err := tc.State().Set("analysis_result", string(resultJSON)); err != nil {
		log.Printf("ERROR: Failed to set analysis_result state: %v", err)
		return nil, err
	}
	if err := tc.State().Set("original_image_url", args.ImageURL); err != nil {
		log.Printf("ERROR: Failed to set original_image_url state: %v", err)
		return nil, err
	}

	// Save metadata for session listing
	now := time.Now()
	if err := tc.State().Set("created_at", now.Format(time.RFC3339)); err != nil {
		log.Printf("WARN: Failed to set created_at state: %v", err)
	}
	if err := tc.State().Set("title", formatSessionTitle(now)); err != nil {
		log.Printf("WARN: Failed to set title state: %v", err)
	}
	if err := tc.State().Set("overall_score", result.OverallScore); err != nil {
		log.Printf("WARN: Failed to set overall_score state: %v", err)
	}

	return result, nil
}

func formatSessionTitle(t time.Time) string {
	loc, _ := time.LoadLocation("Asia/Tokyo")
	if loc != nil {
		t = t.In(loc)
	}
	return t.Format("1月2日 15:04")
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
