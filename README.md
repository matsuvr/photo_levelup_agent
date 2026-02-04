第4回 Agentic AI Hackathon with Google Cloud応募予定
https://zenn.dev/hackathons/google-cloud-japan-ai-hackathon-vol4

  Cloud Run (バックエンドAPI):
  - サービス: photo-coach-api
  - リージョン: us-central1
  - URL: https://photo-coach-api-5ljl72v6pa-uc.a.run.app
  App Hosting (フロントエンド):
  - バックエンドID: photo-coach-frontend
  - リージョン: us-central1
  - URL: https://photo-coach-frontend--ai-hackathon-e04d2.us-central1.hosted.app/



# Photo Coach

## 概要

写真を撮る際に、AIが写真の構図や色合いを分析し、より良い写真を撮るためのアドバイスを提供するアプリです。

## 使用技術

- **フロントエンド**: Next.js、Firebase App Hosting
- **バックエンド**: Go、Cloud Run
- **AIモデル**: Google Cloud Vertex AI
- **データベース**: Firestore
- **認証**: Firebase Authentication
- **デプロイ**: Firebase App Hosting, Google Cloud Run

## 機能

- **写真のアップロード**: ユーザーが写真をアップロードできる
- **AIによる分析**: アップロードされた写真をAIが分析し、構図や色合いのアドバイスを提供
- **アドバイスの表示**: AIの分析結果に基づいたアドバイスをユーザーに表示