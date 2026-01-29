package agent

import (
	"context"
	"os"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"github.com/matsuvr/photo_levelup_agent/backend/internal/tools"
)

const systemInstruction = `あなたは写真指導の専門家AIアシスタント「フォトコーチ」です。
ユーザーの写真スキル向上をサポートします。

## あなたの役割
1. ユーザーがアップロードした写真を分析し、構図、露出、色彩、ライティング、ピント、現像、距離感、意図の明確さ、の8項目を評価
2. 受賞レベルの写真に変換するための改善ポイントを提示
3. 元の写真と改善案を比較し、具体的な改善アドバイスを提供
4. ユーザーの追加質問に対して、文脈を理解した上で詳細に回答

## 利用可能なツール
- analyze_photo: 写真を分析して評価を返す

## 応答ルール
1. 具体的で実践的なアドバイスを提供
2. 専門用語は必要に応じて説明を加える
3. Lightroom/Photoshopの具体的な操作手順を含める
4. カメラ設定は具体的な数値で示す
5. ユーザーの質問には、過去の分析結果を参照しながら回答

## セッション状態の活用
- original_image_url: アップロードされた元画像のURL
- analysis_result: 分析結果のJSON
これらの状態を参照して、一貫性のあるアドバイスを提供してください。
`

func NewPhotoCoachAgent(ctx context.Context) (agent.Agent, error) {
	modelName := os.Getenv("VERTEXAI_LLM")
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if location == "" {
		location = os.Getenv("GOOGLE_CLOUD_REGION")
	}
	if modelName == "" {
		modelName = "gemini-3-pro-preview"
	}

	if err := os.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true"); err != nil {
		return nil, err
	}

	model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, err
	}

	analyzePhotoTool, err := tools.NewAnalyzePhotoTool()
	if err != nil {
		return nil, err
	}
	compareAndAdviseTool, err := tools.NewCompareAndAdviseTool()
	if err != nil {
		return nil, err
	}

	photoAgent, err := llmagent.New(llmagent.Config{
		Name:        "photo_coach",
		Description: "写真スキル向上をサポートするAIコーチ。写真分析と改善アドバイスを行う。",
		Instruction: systemInstruction,
		Model:       model,
		Tools: []tool.Tool{
			analyzePhotoTool,
			compareAndAdviseTool,
		},
	})
	if err != nil {
		return nil, err
	}

	return photoAgent, nil
}
