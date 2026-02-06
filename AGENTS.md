# Agent Guidelines & Project Rules

This document outlines the mandatory rules and best practices for the AI agent working on this project.

gemini-3-pro-image-preview, gemini-3-flash-preview はすでにリリースされているモデルです。勝手にモデル名を書き換えないように！！！！

GoバックエンドがGeminiとやりとりするときは[Google Gen AI Go SDK](https://pkg.go.dev/google.golang.org/genai#section-readme)を用いること。Generative ai sdkはすでに更新されなくなったものなので使わないこと！

## 1. Technology Stack & Tools
- **TypeScript Linting/Formatting**: Use **Biome** explicitly. **Do NOT use ESLint**.
- **TypeScript Types**: Enforce strict typing.
  - **No `any`**: The use of `any` (explicit or implicit) is strictly prohibited.
- **Go**: Use standard Go linters and formatters.

## 2. Implementation Standards
- **Google ADK**: Always refer to the [Google ADK Documentation](https://google.github.io/adk-docs/get-started/go/) and strictly follow its best practices.
- **Design Alignment**: Implement features strictly based on the specifications in `design.md`.
- **Next.js Best Practices**: Adhere to the best practices modeled in the `next-best-practices` skill.

## 3. UI/UX Philosophy
- **Mobile-First**: The application is designed primarily for smartphones.
  - Prioritize mobile usability in all UI/UX decisions.
  - Desktop/PC browser support is a secondary "nice-to-have" (bonus) and should not compromise the mobile experience.

## 4. Session Architecture

### セッションのライフサイクル
- **写真アップロード = 新しいセッション**: フロントエンドで新しい `sessionId` を生成し、バックエンドのADKセッションと紐付ける。
- **フォローアップ質問 = 同一セッション**: 同じ `sessionId` を使ってチャットするため、エージェントは分析結果を踏まえて回答できる。
- **別のセッションには影響しない**: 各セッションは `frontend_session_id` で独立して管理される。

### セッションの流れ
1. **フロントエンド**: 写真アップロード時に `generateSessionId()` で新しいIDを生成
2. **バックエンド (analyze.go)**: Gemini直接呼び出しで分析 → ADKセッションを作成し `frontend_session_id` をstateに保存 → 分析結果をstateに保存
3. **バックエンド (chat.go)**: `resolveSessionID` で `frontend_session_id` からADKセッションを特定 → stateから分析結果を取得しメッセージに付与 → ADK runner経由でエージェント実行
4. **会話履歴**: ADKセッションのイベントにContentをJSON永続化し、`runner.Run` 間で会話が引き継がれる

### 重要な実装ルール
- `processAnalysis` はGemini直接呼び出し（ADK runner経由ではない）なので、分析結果はstateに保存する
- `chatWithAgent` はstateから分析結果を読み取り、ユーザーメッセージにコンテキストとして付与する
- `AppendEvent` / `getEvents` でevent Contentを必ずJSON永続化する（会話履歴が失われるため）

## 5. Project Context (Hackathon)
- **Scope**: This is a submission for a hackathon, not a production-ready commercial product.
- **Backward Compatibility**: **Not required**. Breaking changes are acceptable.
- **Reliability vs. Efficiency**:
  - Service downtime caused by deployment failures is clear and acceptable.
  - **Priority**: Do NOT write wasteful code or consume resources for the sake of redundancy, high availability, or continuity.
  - **Cost**: This is a hobby project; minimize resource consumption and cost. Avoid over-engineering.
