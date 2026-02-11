# Photo Levelup 処理フロー

このドキュメントでは、Photo Levelup アプリケーションにおける画像分析とチャットインタラクションの処理フローを説明します。
スマートフォンでの視認性を高めるため、処理フローを分割して記載しています。

## 1. 画像アップロードと分析ジョブ開始

ユーザーが写真をアップロードし、バックエンドが非同期ジョブを受け付けるまでのフローです。

```mermaid
sequenceDiagram
    participant User as ユーザー
    participant FE as Frontend
    participant API as Backend API

    Note over User, API: 画像アップロード開始
    User->>FE: 写真をアップロード
    FE->>API: POST /photo/analyze (画像データ)
    API->>API: 非同期ジョブ(Goroutine)起動
    API-->>FE: JobIDを返却 (Status: Accepted)
```

## 2. バックグラウンド処理: 分析と改善

非同期ワーカーが画像をGeminiで分析・改善し、Cloud Storageに保存するフローです。

```mermaid
sequenceDiagram
    participant Worker as Async Worker
    participant Gemini as Gemini API
    participant GCS as Cloud Storage

    Note over Worker, GCS: 画像リサイズと保存
    Worker->>Worker: 画像リサイズ
    Worker->>GCS: 元画像をアップロード

    Note over Worker, Gemini: Geminiによる分析
    Worker->>Gemini: AnalyzeImage (直接APIコール)
    Gemini-->>Worker: 分析結果 (JSON)

    Note over Worker, Gemini: 画像の改善
    Worker->>Gemini: EnhancePhoto (直接APIコール)
    Gemini-->>Worker: 改善された画像データ
    Worker->>GCS: 改善画像をアップロード
```

### AIモデル連携詳細

入力画像から採点・講評を経て、添削・お手本画像を作成する詳細フローです。

```mermaid
graph TD
    Input[入力画像] --> Flash[Gemini 3 Flash Preview]
    Flash -->|採点と講評| Grading[採点・講評]
    Grading --> Pro[Gemini 3 Pro Image Preview<br/>(Nanobanana pro)]
    Pro --> Corrected[添削画像]
    Pro --> Example[お手本画像]
```

## 3. バックグラウンド処理: セッション状態の保存

分析結果をADKセッション（Firestore）に保存し、会話の基点となるイベントを記録します。

```mermaid
sequenceDiagram
    participant Worker as Async Worker
    participant ADK as ADK Session (Firestore)

    Note over Worker, ADK: ADKセッションへのコンテキスト注入
    Worker->>ADK: セッション状態を更新 (画像URL, 分析結果)
    Worker->>ADK: イベントシード (ユーザー発言 + モデル要約)
    Worker->>Worker: ジョブを完了状態に更新
```

## 4. ポーリングと結果表示

フロントエンドがジョブの完了を検知し、結果を表示するフローです。

```mermaid
sequenceDiagram
    participant FE as Frontend
    participant API as Backend API
    participant User as ユーザー

    loop ポーリング (分析完了待ち)
        FE->>API: GET /photo/analyze/status?jobId=...
        alt ジョブ完了
            API-->>FE: Status: Completed, Result
        else ジョブ実行中
            API-->>FE: Status: Pending
        end
    end

    FE-->>User: 分析結果と改善画像を表示
```

## 5. チャットインタラクション: 質問送信

ユーザーが分析結果について質問する場合のフローです。

```mermaid
sequenceDiagram
    participant User as ユーザー
    participant FE as Frontend
    participant API as Backend API

    Note over User, API: アドバイスを求める
    User->>FE: 「構図を良くするには？」
    FE->>API: POST /photo/chat (msg, sessionId)
```

## 6. チャットインタラクション: エージェント実行

バックエンドがADKセッションから分析結果を取得し、エージェントを実行して回答を生成するフローです。

```mermaid
sequenceDiagram
    participant API as Backend API
    participant ADK as ADK Session
    participant Agent as Agent Runner

    Note over API, Agent: コンテキスト取得と応答生成
    API->>ADK: セッション状態を取得
    ADK-->>API: 分析コンテキスト (JSON)

    API->>API: メッセージ拡張 (User Msg + Analysis)
    API->>Agent: エージェント実行
    Agent-->>API: レスポンス生成
```
