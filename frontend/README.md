# Photo Levelup Frontend

Firebase App Hosting (Cloud Run) にデプロイできる Next.js フロントエンドです。

## セットアップ

```
cd frontend
npm install
```

`.env.local` にバックエンドの URL を設定してください。

```
BACKEND_BASE_URL=https://your-cloud-run-endpoint
```

## 開発

```
npm run dev
```

## ビルド

```
npm run build
npm run start
```

## デプロイ

> [!CAUTION]
> **絶対に `firebase deploy --only hosting` を実行しないでください！**
>
> このプロジェクトは Firebase App Hosting (Cloud Run) を使用しています。
> 静的ホスティングではありません。

デプロイは自動で行われます：
1. `main` ブランチへプッシュ
2. Firebase App Hosting が `apphosting.yaml` を読み取り自動デプロイ

手動デプロイが必要な場合は Firebase Console から App Hosting を再デプロイしてください。

