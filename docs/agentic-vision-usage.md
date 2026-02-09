# Gemini 3 Flash Preview と Agentic Vision の使用状況

## 概要

本プロジェクト「Photo Levelup Agent」では、Google Gemini 3 Flash Preview (`gemini-3-flash-preview`) を写真分析のメインモデルとして使用しています。しかし、**Agentic Visionの高度な機能（Think-Act-Observeループによる画像の動的操作）は使用しておらず、標準的なマルチモーダルLLMとして活用しています**。

## 使用箇所

### 1. 画像分析 (`backend/internal/services/gemini.go`)

```go
func modelName() string {
    if name := os.Getenv("GEMINI_MODEL"); name != "" {
        return name
    }
    return "gemini-3-flash-preview"
}
```

**場所**: Line 456-460

**用途**: デフォルトの写真分析モデルとして使用

```go
response, err := g.client.Models.GenerateContent(ctx, modelName(), contents, &genai.GenerateContentConfig{
    ResponseMIMEType: "application/json",
    ResponseSchema:   analysisResponseSchema(),
    Tools: []*genai.Tool{
        {CodeExecution: &genai.ToolCodeExecution{}},
    },
})
```

**場所**: Line 206-212

**用途**: 写真を分析し、構図・露出・色彩・ライティング・ピント・現像・距離感・意図の明確さの8項目を評価

**注意**: `CodeExecution` は有効化されていますが、画像の動的操作（ズーム、クロップ、アノテーション）には使用されていません。分析プロンプトがシンプルなテキスト評価であり、Agentic Visionの特徴的な使用方法ではありません。

### 2. 写真比較とアドバイス (`backend/internal/services/gemini.go`)

```go
response, err := g.client.Models.GenerateContent(ctx, modelName(), contents, &genai.GenerateContentConfig{})
```

**場所**: Line 266

**用途**: 元の写真と改善案の写真を比較し、具体的な改善アドバイスを提供

**注意**: 2枚の画像を並べて比較するシンプルな使用方法です。Agentic VisionのThink-Act-Observeループは使用していません。

### 3. ADKエージェント (`backend/internal/agent/photo_coach.go`)

```go
modelName := os.Getenv("GEMINI_MODEL")
if modelName == "" {
    modelName = "gemini-3-flash-preview"
}

model, err := gemini.NewModel(ctx, modelName, &genai.ClientConfig{
    APIKey:  apiKey,
    Backend: genai.BackendGeminiAPI,
})
```

**場所**: Line 53-65

**用途**: 写真コーチエージェントのベースモデルとして使用

**注意**: チャット応答用のLLMとして使用。画像分析ツール（`analyze_photo`）を呼び出すエージェントとして機能しますが、直接Agentic Visionの機能は使用していません。

### 4. 分析ハンドラー (`backend/internal/handlers/analyze.go`)

```go
geminiClient := services.NewGeminiClient()
result, err := geminiClient.AnalyzeImage(ctx, imageURL)
```

**場所**: Line 290-291

**用途**: バックエンドで直接Gemini APIを呼び出して画像分析を実行

**フロー**:
1. ユーザーが写真をアップロード
2. 画像をGCSにアップロード
3. `gemini-3-flash-preview` で写真を分析
4. JSON形式で分析結果を返す

## Agentic Vision との比較

### Agentic Visionの主な機能

Agentic Visionは、画像理解を静的なアクションからエージェントプロセスへと変換します。特徴的な機能は以下の通りです：

1. **Think, Act, Observe ループ**:
   - **Think**: クエリと初期画像を分析し、マルチステップの計画を策定
   - **Act**: Pythonコードを生成・実行して画像を操作（クロッピング、回転、アノテーションなど）
   - **Observe**: 変換された画像をコンテキストに追加し、観察

2. **主な使用例**:
   - **Zooming and inspecting**: 高解像度画像の特定領域を拡大・検査
   - **Image annotation**: 画像に直接描画して推論を視覚的に説明
   - **Visual math and plotting**: 表データの解析と視覚化

3. **効果**: コード実行を有効化することで、ほとんどのビジョンベンチマークで5-10%の品質向上

### 本プロジェクトでの使用状況

| 機能 | Agentic Vision標準 | 本プロジェクト |
|------|-------------------|---------------|
| Think-Act-Observeループ | 使用 | 未使用 |
| 画像の動的操作（ズーム、クロップ） | 使用 | 未使用 |
| 画像アノテーション | 使用 | 未使用 |
| Code Execution有効化 | 必須 | 有効化済み |
| 使用パターン | 能動的な画像調査 | 受動的な画像分析 |

**結論**: 本プロジェクトでは `CodeExecution` は有効化されていますが、Agentic Visionの核心的な機能であるThink-Act-Observeループによる能動的な画像操作は使用していません。標準的なマルチモーダルLLMとして、画像をテキストとして受け取り分析するシンプルなパターンを採用しています。

## 今後の改善案

Agentic Visionの機能を活用して、より高度な写真分析が可能です：

1. **詳細領域の拡大分析**: 構図の問題箇所や露出の不均一な領域を自動検出し、拡大して詳細分析
2. **視覚的なアノテーション**: 改善ポイントを画像に直接マークアップ（赤ペン先生風のアノテーション）
3. **比較分析の強化**: Before/After画像の差異をピクセルレベルで可視化

## 環境変数

`GEMINI_MODEL` 環境変数でモデル名を変更可能：

```bash
# docker-compose.yml (Line 12)
GEMINI_MODEL: "${GEMINI_MODEL:-gemini-3-flash-preview}"
```

## 関連リンク

- [Introducing Agentic Vision in Gemini 3 Flash](https://blog.google/innovation-and-ai/technology/developers-tools/agentic-vision-gemini-3-flash/)
- [Google AI Studio Demo](https://aistudio.google.com/apps/bundled/gemini_visual_thinking)
- [Developer Docs - Code Execution](https://ai.google.dev/gemini-api/docs/code-execution#images)
