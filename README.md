第4回 Agentic AI Hackathon with Google Cloud応募予定
https://zenn.dev/hackathons/google-cloud-japan-ai-hackathon-vol4

https://photo-levelup-frontend-176014985412.asia-northeast1.run.app/

## CI (local pre-push)

GitHub Actions はリソース節約のため、ビルド・デプロイはローカルの pre-push で実行します。
削除のみ GitHub Actions でも実行します。

### セットアップ

```
./scripts/install-pre-push.sh
```

### ローカル実行内容

- `deploy.sh` が実行されます。
- Cloud Run へのデプロイ後、Artifact Registry の `firebaseapphosting-images` と `images` の各リポジトリで、最新以外のイメージを削除します。

### GitHub Actions

- push のたびに Artifact Registry の古いイメージ削除のみ実行します。
