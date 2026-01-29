package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/services"
)

type CompareAndAdviseArgs struct {
	OriginalImageURL    string `json:"original_image_url" desc:"元の画像のCloud Storage URL"`
	TransformedImageURL string `json:"transformed_image_url" desc:"変換後の画像のCloud Storage URL"`
	AnalysisJSON        string `json:"analysis_json,omitempty" desc:"分析結果JSON"`
}

type CompareAndAdviseResult struct {
	Advice string `json:"advice"`
}

func compareAndAdvise(tc tool.Context, args CompareAndAdviseArgs) (*CompareAndAdviseResult, error) {
	ctx := context.Background()

	analysis := args.AnalysisJSON
	if analysis == "" {
		stored, err := tc.State().Get("analysis_result")
		if err == nil {
			if value, ok := stored.(string); ok {
				analysis = value
			}
		}
	}

	service := services.NewGeminiClient()
	advice, err := service.CompareAndAdvise(ctx, args.OriginalImageURL, args.TransformedImageURL, analysis)
	if err != nil {
		return nil, fmt.Errorf("比較アドバイスに失敗しました: %w", err)
	}

	adviceJSON, _ := json.Marshal(advice)
	_ = tc.State().Set("compare_advice", string(adviceJSON))

	return &CompareAndAdviseResult{Advice: advice}, nil
}

func NewCompareAndAdviseTool() (tool.Tool, error) {
	toolInstance, err := functiontool.New(
		functiontool.Config{
			Name:        "compare_and_advise",
			Description: "元の写真と改善案を比較し、具体的な改善ポイントと撮影・現像のアドバイスを生成します。",
		},
		compareAndAdvise,
	)
	if err != nil {
		return nil, err
	}
	return toolInstance, nil
}
