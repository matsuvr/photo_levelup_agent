# 写真上達AIエージェント システム設計書 v2
## ADK Go + Google Cloud 構成

---

## 概要

ユーザーがアップロードした写真を分析し、コンテスト受賞レベルの写真へ変換生成。元写真との比較から具体的な改善アドバイスを提供し、継続的な対話で理解を深める。

**主要な変更点（v1からの変更）:**
- バックエンドを **Go + ADK (Agent Development Kit)** で実装
- マルチターン会話・セッション管理をADKフレームワークに委譲
- カスタムToolとしてNano Banana Pro連携を実装

---

## システムアーキテクチャ

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Frontend (Next.js on firaebase)                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────────┐  │
│  │ 画像アップロード │  │  比較ビューア  │  │      チャットUI              │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                     Cloud Run (ADK Go Backend on cloud run)                          │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │                    ADK Runner + Agent                            │  │
│  │  ┌────────────────────────────────────────────────────────────┐ │  │
│  │  │              Photo Coach Agent (LLM Agent)                 │ │  │
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐ │ │  │
│  │  │  │AnalyzePhoto  │  │TransformPhoto│  │ CompareAndAdvise │ │ │  │
│  │  │  │   Tool       │  │    Tool      │  │      Tool        │ │ │  │
│  │  │  └──────────────┘  └──────────────┘  └──────────────────┘ │ │  │
│  │  └────────────────────────────────────────────────────────────┘ │  │
│  │                              │                                   │  │
│  │  ┌────────────────────────────────────────────────────────────┐ │  │
│  │  │  SessionService          │    MemoryService                │ │  │
│  │  │  (Vertex AI Sessions)    │    (Vertex AI Memory Bank)      │ │  │
│  │  └────────────────────────────────────────────────────────────┘ │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
          │                │                │                │
          ▼                ▼                ▼                ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Google AI    │  │ Google AI    │  │ Cloud        │  │ Vertex AI    │
│ Studio       │  │ Studio       │  │ Storage      │  │ Agent Engine │
│ (Vision)     │  │ (Nano Banana)│  │              │  │ (Session/    │
│              │  │              │  │              │  │  Memory)     │
└──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘
```

---

## ADKを採用する理由

| 課題 | 素朴な実装 | ADK活用 |
|------|-----------|---------|
| マルチターン会話管理 | 自前で履歴保存・コンテキスト構築 | `SessionService`が自動管理 |
| 会話履歴の永続化 | Firestore等に自前実装 | `VertexAiSessionService`で完結 |
| 長期記憶（ユーザー傾向等） | 自前でベクトルDB構築 | `MemoryService`でセマンティック検索 |
| ツール呼び出しの制御 | LLMレスポンスのパース・実行を自前実装 | ADKが自動でツール実行・結果注入 |
| 状態管理 | セッション間のデータ受け渡しを自前実装 | `session.State`で簡単に管理 |

---

## ADK Goコンポーネント構成

```
photo-coach/
├── cmd/
│   └── server/
│       └── main.go              # エントリーポイント
├── internal/
│   ├── agent/
│   │   └── photo_coach.go       # Photo Coach Agent定義
│   ├── tools/
│   │   ├── analyze.go           # 写真分析ツール
│   │   ├── transform.go         # 写真変換ツール (Nano Banana Pro)
│   │   └── compare.go           # 比較・アドバイスツール
│   ├── services/
│   │   ├── gemini.go            # Gemini Client (Google AI Studio)
│   │   ├── nanobanana.go        # Nano Banana Pro クライアント (Google AI Studio)
│   │   └── storage.go           # Cloud Storage クライアント
│   └── api/
│       └── handlers.go          # HTTP ハンドラ (画像アップロード等)
├── go.mod
├── go.sum
└── Dockerfile
```

---

## 処理フロー

### ユーザーインタラクション全体フロー

```
┌─────────┐     ┌──────────────┐     ┌─────────────────────────────────┐
│  User   │────▶│   Frontend   │────▶│         ADK Runner              │
└─────────┘     └──────────────┘     │  ┌─────────────────────────┐   │
                                     │  │   Photo Coach Agent     │   │
    1. 写真アップロード                 │  │                         │   │
    2. 「分析して」と入力               │  │  "この写真を分析して     │   │
                                     │  │   アドバイスください"    │   │
                                     │  │                         │   │
                                     │  │  ┌─────────────────┐   │   │
                                     │  │  │ Tool Selection  │   │   │
                                     │  │  │ by LLM          │   │   │
                                     │  │  └────────┬────────┘   │   │
                                     │  │           │             │   │
                                     │  │     ┌─────▼─────┐       │   │
                                     │  │     │AnalyzeTool│       │   │
                                     │  │     └─────┬─────┘       │   │
                                     │  │           │             │   │
                                     │  │     ┌─────▼─────┐       │   │
                                     │  │     │TransformTool│     │   │
                                     │  │     └─────┬─────┘       │   │
                                     │  │           │             │   │
                                     │  │     ┌─────▼─────┐       │   │
                                     │  │     │CompareTool│       │   │
                                     │  │     └─────┬─────┘       │   │
                                     │  │           │             │   │
                                     │  │  ┌────────▼────────┐   │   │
                                     │  │  │ Generate Response│   │   │
                                     │  │  └─────────────────┘   │   │
                                     │  └─────────────────────────┘   │
                                     │                                │
                                     │  SessionService が自動で       │
                                     │  会話履歴を保存                 │
                                     └─────────────────────────────────┘
```

### フォローアップ質問の処理

```
User: "三分割法についてもっと詳しく教えて"
        │
        ▼
┌─────────────────────────────────────────────────┐
│              ADK Runner                         │
│                                                 │
│  SessionService.GetSession(sessionId)           │
│        │                                        │
│        ▼                                        │
│  ┌─────────────────────────────────────────┐   │
│  │ Session State には:                      │   │
│  │  - original_image_url                   │   │
│  │  - transformed_image_url                │   │
│  │  - analysis_result (JSON)               │   │
│  │  - 過去の会話履歴                         │   │
│  └─────────────────────────────────────────┘   │
│        │                                        │
│        ▼                                        │
│  LLM は過去のコンテキストを理解した上で          │
│  三分割法について詳細に回答                      │
│                                                 │
└─────────────────────────────────────────────────┘
```

---

## コア実装

### 1. エントリーポイント (main.go)

```go
// cmd/server/main.go
package main

import (
	"context"
	"log"
	"os"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"

	"photo-coach/internal/agent/photocoach"
)

func main() {
	ctx := context.Background()

	// 本番環境では Vertex AI のサービスを使用
	var sessionService session.Service
	var memoryService memory.Service

	if os.Getenv("ENV") == "production" {
		// Vertex AI Session Service (永続化)
		sessionService = session.NewVertexAIService(
			os.Getenv("PROJECT_ID"),
			os.Getenv("LOCATION"),
			os.Getenv("AGENT_ENGINE_ID"),
		)
		// Vertex AI Memory Bank Service (長期記憶)
		memoryService = memory.NewVertexAIMemoryBankService(
			os.Getenv("PROJECT_ID"),
			os.Getenv("LOCATION"),
			os.Getenv("AGENT_ENGINE_ID"),
		)
	} else {
		// 開発環境ではインメモリ
		sessionService = session.InMemoryService()
		memoryService = memory.InMemoryService()
	}

	// Photo Coach Agent を作成
	photoCoachAgent, err := photocoach.NewAgent(ctx)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Launcher 設定
	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(photoCoachAgent),
		SessionService: sessionService,
		MemoryService:  memoryService,
	}

	// Web + API モードで起動
	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
```

### 2. Photo Coach Agent 定義

```go
// internal/agent/photocoach/agent.go
package photocoach

import (
	"context"
	"os"

	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"photo-coach/internal/tools"
)

const systemInstruction = `あなたは写真指導の専門家AIアシスタント「フォトコーチ」です。
ユーザーの写真スキル向上をサポートします。

## あなたの役割
1. ユーザーがアップロードした写真を分析し、構図・露出・色彩・ライティングを評価
2. Nano Banana Proを使って、コンテスト受賞レベルの写真に変換
3. 元の写真と変換後の写真を比較し、具体的な改善アドバイスを提供
4. ユーザーの追加質問に対して、文脈を理解した上で詳細に回答

## 利用可能なツール
- analyze_photo: 写真を分析して評価を返す
- transform_photo: 写真をコンテスト受賞レベルに変換
- compare_and_advise: 2枚の写真を比較してアドバイスを生成

## 応答ルール
1. 具体的で実践的なアドバイスを提供
2. 専門用語は必要に応じて説明を加える
3. Lightroom/Photoshopの具体的な操作手順を含める
4. カメラ設定は具体的な数値で示す
5. ユーザーの質問には、過去の分析結果を参照しながら回答

## セッション状態の活用
- original_image_url: アップロードされた元画像のURL
- transformed_image_url: 変換後の画像URL
- analysis_result: 分析結果のJSON
- user_skill_level: ユーザーのスキルレベル（初心者/中級者/上級者）

これらの状態を参照して、一貫性のあるアドバイスを提供してください。
`

func NewAgent(ctx context.Context) (*llmagent.LLMAgent, error) {
	// Gemini モデルの初期化 (Google AI Studio)
	model, err := gemini.NewModel(ctx, "gemini-2.0-flash-exp", &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGoogleAI,
	})
	if err != nil {
		return nil, err
	}

	// カスタムツールの初期化
	analyzePhotoTool := tools.NewAnalyzePhotoTool()
	transformPhotoTool := tools.NewTransformPhotoTool()
	compareAndAdviseTool := tools.NewCompareAndAdviseTool()

	// Agent の作成
	agent, err := llmagent.New(llmagent.Config{
		Name:        "photo_coach",
		Model:       model,
		Description: "写真スキル向上をサポートするAIコーチ。写真分析、変換、アドバイス生成を行う。",
		Instruction: systemInstruction,
		Tools: []tool.Tool{
			analyzePhotoTool,
			transformPhotoTool,
			compareAndAdviseTool,
		},
	})
	if err != nil {
		return nil, err
	}

	return agent, nil
}
```

### 3. 写真分析ツール

```go
// internal/tools/analyze.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/tool"

	"photo-coach/internal/services"
)

// AnalyzePhotoArgs は分析ツールの引数
type AnalyzePhotoArgs struct {
	// ImageURL は分析する画像のCloud Storage URL
	ImageURL string `json:"image_url" desc:"分析する画像のCloud Storage URL (gs://... 形式)"`
}

// AnalysisResult は分析結果
type AnalysisResult struct {
	Composition      CategoryAnalysis `json:"composition"`
	Exposure         CategoryAnalysis `json:"exposure"`
	Color            CategoryAnalysis `json:"color"`
	Lighting         CategoryAnalysis `json:"lighting"`
	TechnicalQuality CategoryAnalysis `json:"technical_quality"`
	OverallScore     int              `json:"overall_score"`
	Summary          string           `json:"summary"`
}

type CategoryAnalysis struct {
	Score        int      `json:"score"`
	Current      string   `json:"current"`
	Issues       []string `json:"issues"`
	ContestLevel string   `json:"contest_level"`
}

// analyzePhoto は写真を分析するツール関数
func analyzePhoto(tc tool.Context, args AnalyzePhotoArgs) (*AnalysisResult, error) {
	ctx := context.Background()

	// Google AI Studio (Vision) で分析
	geminiClient := services.NewGeminiClient()
	result, err := geminiClient.AnalyzeImage(ctx, args.ImageURL)
	if err != nil {
		return nil, fmt.Errorf("画像分析に失敗しました: %w", err)
	}

	// 分析結果をセッション状態に保存
	resultJSON, _ := json.Marshal(result)
	tc.State().Set("analysis_result", string(resultJSON))
	tc.State().Set("original_image_url", args.ImageURL)

	return result, nil
}

// NewAnalyzePhotoTool は分析ツールを作成
func NewAnalyzePhotoTool() tool.Tool {
	return tool.NewFunctionTool(
		"analyze_photo",
		"写真を詳細に分析し、構図・露出・色彩・ライティング・技術的品質を評価します。"+
			"各項目について10点満点でスコアリングし、改善点とコンテスト入賞レベルに必要な要素を提示します。",
		analyzePhoto,
	)
}
```

### 4. 写真変換ツール (Nano Banana Pro)

```go
// internal/tools/transform.go
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/adk/tool"

	"photo-coach/internal/services"
)

// TransformPhotoArgs は変換ツールの引数
type TransformPhotoArgs struct {
	// ImageURL は変換する画像のURL
	ImageURL string `json:"image_url" desc:"変換する画像のCloud Storage URL"`
	// AnalysisJSON は分析結果のJSON（オプション）
	AnalysisJSON string `json:"analysis_json,omitempty" desc:"analyze_photoツールの結果JSON。省略時はセッション状態から取得"`
}

// TransformResult は変換結果
type TransformResult struct {
	TransformedImageURL string   `json:"transformed_image_url"`
	Changes             []string `json:"changes"`
	Reasoning           string   `json:"reasoning"`
}

func transformPhoto(tc tool.Context, args TransformPhotoArgs) (*TransformResult, error) {
	ctx := context.Background()

	// 分析結果を取得（引数から、またはセッション状態から）
	analysisJSON := args.AnalysisJSON
	if analysisJSON == "" {
		if stored, err := tc.State().Get("analysis_result"); err == nil {
			analysisJSON = stored.(string)
		}
	}

	if analysisJSON == "" {
		return nil, fmt.Errorf("分析結果がありません。先にanalyze_photoを実行してください")
	}

	var analysis AnalysisResult
	if err := json.Unmarshal([]byte(analysisJSON), &analysis); err != nil {
		return nil, fmt.Errorf("分析結果のパースに失敗: %w", err)
	}

	// Nano Banana Pro で変換
	nanoBananaClient := services.NewNanoBananaClient()
	result, err := nanoBananaClient.TransformImage(ctx, args.ImageURL, &analysis)
	if err != nil {
		return nil, fmt.Errorf("画像変換に失敗しました: %w", err)
	}

	// 変換結果をセッション状態に保存
	tc.State().Set("transformed_image_url", result.TransformedImageURL)

	return result, nil
}

// NewTransformPhotoTool は変換ツールを作成
func NewTransformPhotoTool() tool.Tool {
	return tool.NewFunctionTool(
		"transform_photo",
		"分析結果に基づいて、写真をコンテスト受賞レベルに変換します。"+
			"Nano Banana Pro（Gemini 3 Pro Image）を使用して、構図調整、露出補正、色彩調整、"+
			"ライティング改善を行います。変換後の画像URLと、行った変更の詳細を返します。",
		transformPhoto,
	)
}
```

### 5. 比較・アドバイスツール

```go
// internal/tools/compare.go
package tools

import (
	"context"
	"fmt"

	"google.golang.org/adk/tool"

	"photo-coach/internal/services"
)

// CompareAndAdviseArgs は比較ツールの引数
type CompareAndAdviseArgs struct {
	// OriginalImageURL は元画像のURL（省略時はセッション状態から取得）
	OriginalImageURL string `json:"original_image_url,omitempty" desc:"元画像のURL"`
	// TransformedImageURL は変換後画像のURL（省略時はセッション状態から取得）
	TransformedImageURL string `json:"transformed_image_url,omitempty" desc:"変換後画像のURL"`
}

// AdviceResult はアドバイス結果
type AdviceResult struct {
	Comparison struct {
		Composition string `json:"composition"`
		Exposure    string `json:"exposure"`
		Color       string `json:"color"`
		Lighting    string `json:"lighting"`
	} `json:"comparison"`
	ShootingTips       []string `json:"shooting_tips"`
	PostProcessingTips []string `json:"post_processing_tips"`
	EquipmentTips      []string `json:"equipment_tips,omitempty"`
	PracticeExercises  []string `json:"practice_exercises"`
	SuggestedQuestions []string `json:"suggested_questions"`
}

func compareAndAdvise(tc tool.Context, args CompareAndAdviseArgs) (*AdviceResult, error) {
	ctx := context.Background()

	// URLをセッション状態から取得（引数が空の場合）
	originalURL := args.OriginalImageURL
	transformedURL := args.TransformedImageURL

	if originalURL == "" {
		if stored, err := tc.State().Get("original_image_url"); err == nil {
			originalURL = stored.(string)
		}
	}
	if transformedURL == "" {
		if stored, err := tc.State().Get("transformed_image_url"); err == nil {
			transformedURL = stored.(string)
		}
	}

	if originalURL == "" || transformedURL == "" {
		return nil, fmt.Errorf("比較する画像がありません。先にanalyze_photoとtransform_photoを実行してください")
	}

	// Vertex AI で比較分析
	geminiClient := services.NewGeminiClient()
	result, err := geminiClient.CompareAndGenerateAdvice(ctx, originalURL, transformedURL)
	if err != nil {
		return nil, fmt.Errorf("比較分析に失敗しました: %w", err)
	}

	return result, nil
}

// NewCompareAndAdviseTool は比較・アドバイスツールを作成
func NewCompareAndAdviseTool() tool.Tool {
	return tool.NewFunctionTool(
		"compare_and_advise",
		"元の写真と変換後の写真を比較し、具体的な改善アドバイスを生成します。"+
			"撮影時のコツ、現像・レタッチの手順（Lightroom/Photoshopの具体的な値）、"+
			"おすすめの練習方法を提供します。",
		compareAndAdvise,
	)
}
```

### 6. Nano Banana Pro クライアント

```go
// internal/services/nanobanana.go
package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
	"google.golang.org/genai"
)

type NanoBananaClient struct {
	client        *genai.Client
	storageClient *storage.Client
	bucketName    string
}

func NewNanoBananaClient() *NanoBananaClient {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGoogleAI,
	})
	if err != nil {
		panic(err)
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	return &NanoBananaClient{
		client:        client,
		storageClient: storageClient,
		bucketName:    os.Getenv("BUCKET_NAME"),
	}
}

func (c *NanoBananaClient) TransformImage(
	ctx context.Context,
	imageURL string,
	analysis *AnalysisResult,
) (*TransformResult, error) {
	// 変換プロンプトを構築
	prompt := c.buildTransformPrompt(analysis)

	// Nano Banana Pro (gemini-3-pro-image-preview) を使用
	model := c.client.GenerativeModel("gemini-3-pro-image-preview")

	// 画像生成設定
	model.GenerationConfig = &genai.GenerationConfig{
		ResponseModalities: []string{"TEXT", "IMAGE"},
		ImageConfig: &genai.ImageConfig{
			AspectRatio: "AUTO", // 元画像のアスペクト比を維持
			ImageSize:   "2K",
		},
	}

	// リクエスト送信
	resp, err := model.GenerateContent(ctx, genai.FileData{
		MIMEType: "image/jpeg",
		FileURI:  imageURL,
	}, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	// レスポンスから画像とテキストを抽出
	var imageData string
	var textContent string

	for _, part := range resp.Candidates[0].Content.Parts {
		switch p := part.(type) {
		case genai.Blob:
			imageData = base64.StdEncoding.EncodeToString(p.Data)
		case genai.Text:
			textContent += string(p)
		}
	}

	// Cloud Storage に保存
	transformedURL, err := c.saveImageToStorage(ctx, imageData)
	if err != nil {
		return nil, err
	}

	return &TransformResult{
		TransformedImageURL: transformedURL,
		Changes:             extractChanges(textContent),
		Reasoning:           textContent,
	}, nil
}

func (c *NanoBananaClient) buildTransformPrompt(analysis *AnalysisResult) string {
	improvements := []string{}

	if analysis.Composition.Score < 8 {
		improvements = append(improvements,
			fmt.Sprintf("構図の改善: %s", analysis.Composition.ContestLevel))
	}
	if analysis.Exposure.Score < 8 {
		improvements = append(improvements,
			fmt.Sprintf("露出の改善: %s", analysis.Exposure.ContestLevel))
	}
	if analysis.Color.Score < 8 {
		improvements = append(improvements,
			fmt.Sprintf("色彩の改善: %s", analysis.Color.ContestLevel))
	}
	if analysis.Lighting.Score < 8 {
		improvements = append(improvements,
			fmt.Sprintf("ライティングの改善: %s", analysis.Lighting.ContestLevel))
	}

	return fmt.Sprintf(`あなたはプロの写真レタッチャーです。
この写真を、写真コンテストで受賞できるレベルに変換してください。

## 現状の分析結果
- 構図スコア: %d/10
- 露出スコア: %d/10
- 色彩スコア: %d/10
- ライティングスコア: %d/10

## 改善すべきポイント
%s

## 変換ルール
1. 被写体の本質的な内容は変えない
2. 構図は、可能な範囲でクロップ・傾き補正で改善
3. 露出は、ハイライト・シャドウ・コントラストを最適化
4. 色彩は、より印象的で調和の取れた色合いに調整
5. 必要に応じて、ビネット効果や部分的な調整を追加
6. 自然な仕上がりを維持し、過度な加工は避ける

変換後の画像を生成し、行った変更点を説明してください。`,
		analysis.Composition.Score,
		analysis.Exposure.Score,
		analysis.Color.Score,
		analysis.Lighting.Score,
		formatImprovements(improvements),
	)
}

func (c *NanoBananaClient) saveImageToStorage(ctx context.Context, base64Data string) (string, error) {
	// base64データをデコードしてCloud Storageに保存
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", err
	}

	objectName := fmt.Sprintf("transformed/%s.jpg", generateUUID())
	obj := c.storageClient.Bucket(c.bucketName).Object(objectName)

	w := obj.NewWriter(ctx)
	w.ContentType = "image/jpeg"

	if _, err := w.Write(data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	return fmt.Sprintf("gs://%s/%s", c.bucketName, objectName), nil
}
```

### 7. HTTP API ハンドラ（画像アップロード用）

```go
// internal/api/handlers.go
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

type UploadHandler struct {
	storageClient *storage.Client
	bucketName    string
}

func NewUploadHandler() *UploadHandler {
	client, _ := storage.NewClient(nil)
	return &UploadHandler{
		storageClient: client,
		bucketName:    os.Getenv("BUCKET_NAME"),
	}
}

// HandleUpload は画像アップロードを処理
// POST /api/upload
func (h *UploadHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// マルチパートフォームをパース（最大20MB）
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Cloud Storage にアップロード
	objectName := fmt.Sprintf("uploads/%s-%s", uuid.New().String(), header.Filename)
	ctx := r.Context()

	obj := h.storageClient.Bucket(h.bucketName).Object(objectName)
	writer := obj.NewWriter(ctx)
	writer.ContentType = header.Header.Get("Content-Type")

	if _, err := io.Copy(writer, file); err != nil {
		http.Error(w, "Failed to upload", http.StatusInternalServerError)
		return
	}

	if err := writer.Close(); err != nil {
		http.Error(w, "Failed to finalize upload", http.StatusInternalServerError)
		return
	}

	// 署名付きURLを生成（フロントエンド表示用）
	signedURL, _ := storage.SignedURL(h.bucketName, objectName, &storage.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(24 * time.Hour),
	})

	response := map[string]string{
		"image_url":  fmt.Sprintf("gs://%s/%s", h.bucketName, objectName),
		"signed_url": signedURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
```

---

## セッション状態の活用

ADKのセッション状態を使って、ツール間でデータを共有:

```go
// ツール内でのセッション状態の読み書き

// 書き込み
tc.State().Set("original_image_url", imageURL)
tc.State().Set("analysis_result", analysisJSON)
tc.State().Set("user:skill_level", "intermediate") // user: プレフィックスで永続化

// 読み込み
if url, err := tc.State().Get("original_image_url"); err == nil {
    originalURL = url.(string)
}

// ユーザーレベルに基づいた回答調整
if level, err := tc.State().Get("user:skill_level"); err == nil {
    // 初心者には基本的な説明を追加
    // 上級者には専門的な内容を提供
}
```

### 状態のスコープ

| プレフィックス | スコープ | 用途 |
|---------------|---------|------|
| (なし) | 現在のセッションのみ | 分析結果、画像URL |
| `user:` | 同一ユーザーの全セッション | スキルレベル、好みの設定 |
| `app:` | 全ユーザー共通 | アプリ設定 |
| `temp:` | 一時的 | ツール間のデータ受け渡し |

---

## 長期記憶（Memory）の活用

ユーザーの写真傾向やよく改善される点を記憶:

```go
// セッション終了時にメモリに保存
func (a *PhotoCoachAgent) OnSessionEnd(ctx context.Context, session *session.Session) {
    // 今回のセッションからキーとなる情報を抽出し、メモリに追加
    memoryService.AddSessionToMemory(session)
}

// 新しいセッション開始時にメモリを検索
func searchUserHistory(tc tool.Context, query string) ([]string, error) {
    results, err := tc.Memory().Search(query)
    if err != nil {
        return nil, err
    }

    var memories []string
    for _, r := range results {
        memories = append(memories, r.Content)
    }
    return memories, nil
}
```

これにより、例えば:
- 「このユーザーは構図の改善が多い」→ 構図についてより詳しく説明
- 「前回は風景写真だったが、今回はポートレート」→ ジャンルの違いを考慮

---

## デプロイ構成

### Dockerfile

```dockerfile
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /photo-coach ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /photo-coach .

EXPOSE 8080
CMD ["./photo-coach", "web", "api"]
```

### Cloud Run 設定 (Terraform)

```hcl
resource "google_cloud_run_v2_service" "photo_coach" {
  name     = "photo-coach"
  location = var.region

  template {
    containers {
      image = "${var.region}-docker.pkg.dev/${var.project_id}/photo-coach/api:latest"

      resources {
        limits = {
          cpu    = "2"
          memory = "2Gi"
        }
      }

      env {
        name  = "ENV"
        value = "production"
      }
      env {
        name  = "PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "LOCATION"
        value = var.region
      }
      env {
        name  = "GEMINI_API_KEY"
        value = var.gemini_api_key
      }
      env {
        name  = "BUCKET_NAME"
        value = google_storage_bucket.photos.name
      }
      env {
        name  = "AGENT_ENGINE_ID"
        value = google_vertex_ai_agent_engine.photo_coach.id
      }
      env {
        name  = "PROJECT_ID"
        value = var.project_id
      }
    }

    service_account = google_service_account.photo_coach.email
  }
}

# Vertex AI Agent Engine（セッション・メモリ管理用）
resource "google_vertex_ai_agent_engine" "photo_coach" {
  project      = var.project_id
  location     = var.region
  display_name = "photo-coach-engine"
}
```

---

## フロントエンド統合

ADK Go は `web api` モードで起動すると REST API を提供します:

```typescript
// フロントエンドからの呼び出し例

// 1. セッション作成
const session = await fetch('/api/sessions', {
  method: 'POST',
  body: JSON.stringify({ user_id: userId, app_name: 'photo_coach' })
}).then(r => r.json());

// 2. 画像アップロード（カスタムエンドポイント）
const uploadRes = await fetch('/api/upload', {
  method: 'POST',
  body: formData
}).then(r => r.json());

// 3. エージェントにメッセージ送信
const response = await fetch(`/api/sessions/${session.id}/messages`, {
  method: 'POST',
  body: JSON.stringify({
    content: `この写真を分析してください: ${uploadRes.image_url}`
  })
}).then(r => r.json());

// 4. フォローアップ質問
const followUp = await fetch(`/api/sessions/${session.id}/messages`, {
  method: 'POST',
  body: JSON.stringify({
    content: '三分割法についてもっと詳しく教えてください'
  })
}).then(r => r.json());
```

---

## まとめ: ADK採用のメリット

| 機能 | ADKなし | ADKあり |
|------|---------|---------|
| 会話履歴管理 | 自前でFirestore実装 | `SessionService`で自動 |
| コンテキスト構築 | 毎回プロンプトに履歴を結合 | ADKが自動で管理 |
| ツール実行 | LLMレスポンスをパースして手動実行 | ADKが自動実行 |
| 状態管理 | 自前でKVストア実装 | `session.State`で簡単 |
| 長期記憶 | ベクトルDB構築・検索実装 | `MemoryService`で完結 |
| 開発UI | 自前で作成 | `adk web` でデバッグUI提供 |
| 本番デプロイ | - | Vertex AI Agent Engineと連携 |

**結論**: ADKを使うことで、マルチターン会話・セッション管理・状態管理といった「面倒だが重要な部分」をフレームワークに任せ、写真分析・変換・アドバイス生成というコアロジックに集中できます。

