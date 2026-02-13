# Google Cloud Japan AI Hackathon Vol.4 応募記事構成案

## 1. タイトル案
インパクト重視で、技術要素と解決課題を明確にします。
- **案A（キャッチー）**: 「ここ直して！」が写真に書き込まれる📸 辛口AIフォトコーチを作ってみた【Go ADK + Gemini 1.5/2.0】
- **案B（技術重視）**: Go製GenAI Agentフレームワーク「ADK」で実装する、マルチモーダル写真指導AI
- **案C（シンプル）**: Photo Levelup: あなたの専属AI写真コーチ

---

## 2. はじめに (Introduction)
### 課題提起
- 写真を撮っても「なんかイマイチ」だけど、どこを直せばいいかわからない。
- SNSに上げても「いいね」はつくけど、具体的な改善点は誰も教えてくれない。
- プロの添削はハードルが高い。

### 解決策 (What we built)
- **Photo Levelup**: 写真をアップロードするだけで、AIが「構図」「露出」「色彩」などを辛口採点。
- **最大の特徴**: 言葉だけでなく、**「ここをこう直せ」という赤ペン先生のような書き込み（Visual Feedback）**を画像生成で行う。
- 対話型AIコーチが、撮影意図を汲み取ってアドバイスしてくれる。

---

## 3. デモ・機能紹介 (Demo & Features)
※ ここには実際のスクリーンショットやGIF動画を配置することを推奨します。

1.  **レーダーチャート分析**:
    - 構図、露出、色彩、ライティングなど8項目を10点満点で評価。
    - 一目で自分の弱点がわかる。
2.  **赤ペン先生機能 (The "Red Pen" Enhancement)**:
    - AIが改善点を指摘するだけでなく、元画像に「矢印」や「丸」を書き込んで視覚的にアドバイス。
    - 「この余白が広すぎる」「ここに視線誘導が必要」などが直感的にわかる。
3.  **AIコーチとの対話**:
    - 「もっとふんわり撮りたいんだけど？」といった抽象的な質問にも、元画像と分析結果を踏まえて回答。

---

## 4. システムアーキテクチャ (Architecture)

### 全体構成図
```mermaid
graph LR
    User[User] --> NextJS[Frontend (Next.js)]
    NextJS --> CloudRun[Backend (Go on Cloud Run)]
    CloudRun --> Gemini[Gemini 1.5 Flash / 2.0 Flash]
    CloudRun --> GCS[Cloud Storage (Images)]
    CloudRun --> Firestore[Firestore (Session State)]
```

### 非同期処理フロー (Async Job System)
画像分析と生成は時間がかかるため、非同期ジョブとして実装しています。
1.  **Frontend**: 画像アップロード -> `jobId` を受け取る (202 Accepted)
2.  **Backend**:
    - 画像のリサイズ & GCSアップロード
    - **Analysis**: `gemini-3-flash-preview` で8項目評価 (JSON出力)
    - **Enhancement**: `gemini-3-pro-image-preview` で「赤ペン画像」と「理想的な完成画像」を並列生成
    - **State Update**: 結果を `Firestore` のセッション状態に保存
3.  **Frontend**: ポーリングで完了を検知 -> 結果表示

---

## 5. 技術的なこだわり (Technical Deep Dive)

### (1) Go言語製 Agent Framework「ADK」の採用
Google純正の **Agent Development Kit (ADK)** (`google.golang.org/adk`) を採用しました。
- **採用理由**: ステートフルな会話管理、ツール呼び出し、コンテキスト維持が容易。
- **実装ポイント**:
    - `Session Service` としてFirestoreを使用し、会話履歴を永続化。
    - `State` 機能を使って、分析結果（JSON）や画像URLをツール間で共有。

### (2) セッションの「事前注入 (Seeding)」テクニック
ユーザーがチャットを開始する前に、AIがすでに写真を「見ている」状態を作るための工夫です。
- **課題**: 通常のチャットボットは「こんにちは」から始まるが、今回は「分析結果」を踏まえた会話にしたい。
- **解決策**: 分析ジョブ完了時に、バックエンド側で**「ユーザーが写真を送り、AIが分析結果を返した」という会話履歴（Event）を強制的に注入**しています。
- これにより、ユーザーがチャット画面を開いた瞬間から、AIは「先ほどの写真の件ですが…」と文脈を理解した状態で話せます。

```go
// 擬似コード
sessionService.AppendEvent(ctx, sessionID, &Event{
    Role: "model",
    Content: "分析完了しました。構図スコアは7点です...",
})
```

### (3) Gemini モデルの使い分け
適材適所でモデルを切り替えています。
- **Gemini 3 Flash (Preview)**:
    - チャット応答、JSON分析（速い、安い、十分賢い）。
    - 8項目の詳細スコアリングに使用。
- **Gemini 3 Pro Image (Preview)**:
    - 「赤ペン先生」画像の生成。
    - **Prompt Engineering**: 「あなたは写真講師です。改善点に赤ペンで書き込みを入れてください」と指示することで、マルチモーダル生成能力を活用。

---

## 6. 苦労した点 (Challenges)
- **画像生成のコントロール**: 「改善して」と言うと別人のような写真になってしまう問題。
    - -> プロンプトで「被写体の保持」を強く指示し、あくまで「補正」に留めるよう調整。
- **非同期処理のUX**: 生成に数秒〜数十秒かかるため、フロントエンドでの待ち時間体験（ローディング表示など）を工夫。

---

## 7. 今後の展望 (Future Work)
- **リアルタイム動画指導**: カメラを向けただけで構図アドバイス。
- **SNS連携**: 改善前/改善後を並べてシェアする機能。
- **スタイル学習**: ユーザーの好きな写真家を学習して、そのスタイルに近づけるアドバイス。

---

## 8. おわりに
Photo Levelupは、単なるフィルターアプリではなく、「撮り手のスキルを向上させる」ことを目指したAIエージェントです。
Geminiのマルチモーダル能力とGo ADKの堅牢なアーキテクチャにより、実用的な写真指導システムを実現できました。

---

### 付録: 技術スタック一覧
- **Frontend**: Next.js, TypeScript, Tailwind CSS
- **Backend**: Go 1.25, Google Cloud Run
- **AI**: Gemini 3 Flash/Pro Image (Preview)
- **Libraries**: `google.golang.org/adk`, `google.golang.org/genai`
- **Infra**: Firebase App Hosting, Cloud Storage, Firestore
