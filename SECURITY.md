# セキュリティガイドライン - APIキー・認証情報の保護

## 重大事項（CRITICAL）

### 絶対にログに出力してはいけないもの
- **Google APIキー**（AIzaSyで始まる）
- **Firebase設定**（apiKeyを含む）
- **認証トークン**（Bearerトークン、JWT、アクセストークン、リフレッシュトークン）
- **パスワード・クレデンシャル**
- **秘密鍵・証明書**（RSA、EC、OPENSSHなど）
- **クレジットカード番号**
- **サービスアカウントキー**

## 実施済みの対策

### 1. .gitignoreの更新（完了）
以下のファイル・ディレクトリを除外対象に追加：
- `firebase-debug.log`
- `firebase-debug.*.log`
- `.firebase/`
- `.firebaserc`
- 各種認証ファイル（*.key, *.pem, .env*）

### 2. ログ出力ロジックの見直し（完了）
- ✅ 既存のログファイル削除済み
- ✅ 安全なログユーティリティ（`secure-log.ts`）を作成
- ✅ APIルートのconsole.log/errorをsecureLogに置き換え済み
- ✅ クライアントコンポーネントのログ出力を置き換え済み

### 3. 安全なログユーティリティ（secure-log.ts）
自動的に以下をマスキング：
- Google APIキー（AIzaSy...）→ `[GOOGLE_API_KEY_MASKED]`
- Bearerトークン → `[BEARER_TOKEN_MASKED]`
- JWTトークン → `[JWT_MASKED]`
- パスワード → `[PASSWORD_MASKED]`
- 秘密鍵 → `[PRIVATE_KEY_MASKED]`

使用方法：
```typescript
import { secureLog } from "@/lib/secure-log"

// 安全なログ出力（自動的に機密情報をマスキング）
secureLog.info("メッセージ", data)
secureLog.error("エラー", error)
secureLog.warn("警告", data)
secureLog.debug("デバッグ", data)

// 生のログ出力（緊急時のみ使用）
secureLog.raw.error("緊急エラー", data)
```

## 継続的な注意事項

### Firebaseデプロイ時の注意
Firebase CLIのデバッグログ（`firebase-debug.log`）にはAPIキーなどの機密情報が含まれる可能性があります：
- デプロイ後は必ず `firebase-debug.log` が生成されていないか確認
- 生成されている場合は削除してからコミット

### 推奨するFirebaseデプロイ手順
```bash
# 1. デプロイ
firebase deploy

# 2. デバッグログの確認と削除
rm -f firebase-debug.log

# 3. Gitステータス確認（余計なファイルがないか）
git status

# 4. コミット
git add .
git commit -m "デプロイ"
```

### 環境変数の管理
- 本番環境の機密情報は `.env.local` に保存（既に.gitignoreに含まれている）
- `.env.example` にはダミー値を記載し、どの環境変数が必要かを示す
- クライアントサイドに露出する環境変数は `NEXT_PUBLIC_` プレフィックスを使用

### コードレビュー時のチェックポイント
- [ ] 新しい `console.log/error/warn` が追加されていないか
- [ ] 環境変数が直接ログに出力されていないか
- [ ] APIレスポンスに機密情報が含まれていないか
- [ ] エラーメッセージにスタックトレースや内部情報が含まれすぎていないか

## インシデント発生時の対応手順

1. **即座にAPIキーを無効化**（Google Cloud Consoleで）
2. **新しいAPIキーを生成**
3. **環境変数を更新**（ローカルと本番）
4. **ログファイルを削除**（`rm -f firebase-debug.log`）
5. **.gitignoreが正しく設定されているか確認**
6. **Gitの履歴から機密情報を削除**（必要に応じて）
7. **コミットしてプッシュ**
8. **デプロイし直す**

## 参考リンク
- [Google Cloud: APIキーのセキュリティ](https://cloud.google.com/docs/security/api-keys)
- [Firebase: セキュリティチェックリスト](https://firebase.google.com/support/guides/security-checklist)
- [GitHub: 機密情報の削除](https://docs.github.com/ja/authentication/keeping-your-account-and-data-secure/removing-sensitive-data-from-a-repository)
