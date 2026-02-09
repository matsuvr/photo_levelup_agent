# Agentic Vision 活用アイデア集

## はじめに

このドキュメントは、Photo Levelup AgentにAgentic VisionのThink-Act-Observeループを取り入れ、より高度な写真分析機能を実現するためのアイデア集です。ハッカソン応募用の趣味プロジェクトとして、技術的な興味を優先した過剰な機能も含めて提案します。

---

## 1. 自動ズーム＆詳細分析（Zooming and inspecting）

### 概要
画像の問題箇所を自動検出し、その領域を拡大して詳細分析を行います。

### ユースケース
- **構図の問題箇所**: 不要な要素が写り込んでいる領域を検出→拡大→改善アドバイス
- **露出の問題**: 白飛び・黒飛びしている領域を検出→拡大→具体的な修正方法
- **ノイズ分析**: 高ISOノイズが多い暗部を検出→拡大→ノイズ低減のアドバイス

### 実装イメージ

```python
# Agentic Visionが生成するコードの例
from PIL import Image, ImageDraw, ImageEnhance
import numpy as np

# 画像を読み込み
img = Image.open("photo.jpg")
img_array = np.array(img)

# 白飛び箇所を検出（明度が高い領域）
white_flying_mask = np.all(img_array > [250, 250, 250], axis=2)

# 白飛び領域のバウンディングボックスを抽出
if np.any(white_flying_mask):
    ys, xs = np.where(white_flying_mask)
    x_min, x_max = xs.min(), xs.max()
    y_min, y_max = ys.min(), ys.max()
    margin = 50
    crop = img.crop((max(0, x_min-margin), max(0, y_min-margin),
                     min(img.width, x_max+margin), min(img.height, y_max+margin)))
    crop.save("zoomed_highlight.jpg")  # 拡大画像を保存

# 分析結果
result = {
    "detected_issue": "白飛び",
    "location": (x_min, y_min, x_max, y_max),
    "zoomed_image": "zoomed_highlight.jpg",
    "advice": "この領域は露出オーバーしています。NDフィルターの使用や、露出補正を-0.7EVに調整することで改善できます。"
}
```

### ユーザー体験
```
ユーザー: 「この写真の問題点を教えて」

システム:
📸 **分析結果**

問題点1: 白飛び (右上の空)
  [拡大画像] ⬆️ 露出がオーバーしています。NDフィルターを使用するか、-0.7EVの露出補正を推奨します。

問題点2: 構図のノイズ (左下の人)
  [拡大画像] ⬆️ 不要な要素が写り込んでいます。少し下を向いて撮影するか、角度を変えるとスッキリします。
```

---

## 2. 赤ペン先生スタイルのアノテーション（Image annotation）

### 概要
写真の改善ポイントを、赤ペン先生風に直接画像に書き込みます。

### ユースケース
- **構図の改善**: 三分割法のラインを描画、主要被写体を丸で囲む
- **視線の誘導**: ユーザーの目線を矢印で示す
- **ピントの確認**: ピントが合っている範囲をハイライト

### 実装イメージ

```python
from PIL import Image, ImageDraw, ImageFont

img = Image.open("photo.jpg")
draw = ImageDraw.Draw(img)

# 赤ペン風のアノテーション
font = ImageFont.truetype("handwriting_font.ttf", 40)

# 1. 三分割法のラインを描画
w, h = img.size
draw.line([(w/3, 0), (w/3, h)], fill="red", width=3)
draw.line([(2*w/3, 0), (2*w/3, h)], fill="red", width=3)
draw.line([(0, h/3), (w, h/3)], fill="red", width=3)
draw.line([(0, 2*h/3), (w, 2*h/3)], fill="red", width=3)

# 2. 主要被写体を丸で囲む（AIが検出した位置）
draw.ellipse([150, 200, 450, 500], outline="red", width=5)
draw.text((160, 180), "ここに注目!", fill="red", font=font)

# 3. 改善アドバイスを書き込む
draw.text((50, 550), "↑ カメラを少し右にずすとバランス良い", fill="red", font=font)
draw.text((400, 100), "↓ 空を増やすと開放感が増します", fill="red", font=font)

# 矢印を描画
draw.line([(480, 600), (580, 650)], fill="red", width=4, arrow="last")

img.save("annotated.jpg")
```

### ユーザー体験
```
[画像表示：赤ペンでアノテーションされた写真]

💡 講評:

「構図は全体的に良いですが、三分割法のラインを意識するとよりプロっぽくなります。

主要被写体（赤丸）が左下の交点にあるのは良いポイント！
ただし、空を少し多く撮ると開放感が増します。

→ 赤い矢印の方向にカメラを少し動かしてみてください！」
```

---

## 3. Before/Afterの差異可視化

### 概要
元写真と改善後の写真をピクセルレベルで比較し、どこがどう変わったかを可視化します。

### ユースケース
- **露出の変化**: 明るくなった・暗くなった領域を色分け
- **色の変化**: 色温度、彩度の変化をヒートマップで表示
- **構図の変化**: クロップの領域を可視化

### 実装イメージ

```python
import numpy as np
import matplotlib.pyplot as plt
from PIL import Image

orig = np.array(Image.open("original.jpg"))
enhanced = np.array(Image.open("enhanced.jpg"))

# 明るさの差を計算
diff_brightness = np.mean(enhanced, axis=2) - np.mean(orig, axis=2)

# ヒートマップを作成
plt.figure(figsize=(12, 6))
plt.subplot(1, 3, 1)
plt.title("Before")
plt.imshow(orig)

plt.subplot(1, 3, 2)
plt.title("After")
plt.imshow(enhanced)

plt.subplot(1, 3, 3)
plt.title("露出の変化")
# 赤=明るくなった、青=暗くなった
plt.imshow(diff_brightness, cmap='RdBu_r', vmin=-50, vmax=50)
plt.colorbar(label="明るさの変化")

plt.tight_layout()
plt.savefig("comparison.png")
```

### ユーザー体験
```
[画像表示: Before / After / 差異ヒートマップ]

📊 **改善点の可視化**

- 露出: 全体的に+15%明るくなりました（特に背景部分）
- 色温度: ウォームトーンに補正（+200K）
- コントラスト: +20%向上

[ヒートマップ] 赤い領域 = 明るくなった場所
```

---

## 4. 視線の流れの可視化

### 概要
写真を見たユーザーの視線がどこに誘導されるかをシミュレーションします。

### ユースケース
- **視線の分析**: どこからどこへ視線が動くかを矢印で示す
- **注意の分散**: 視線が散漫になっている箇所を特定

### 実装イメージ

```python
from PIL import Image, ImageDraw, ImageFont
import numpy as np

img = Image.open("photo.jpg")
draw = ImageDraw.Draw(img)

# コントラストの高い領域（視線が吸い寄せられる場所）を検出
img_gray = np.array(img.convert('L'))
sobel_x = np.abs(np.gradient(img_gray, axis=1))
sobel_y = np.abs(np.gradient(img_gray, axis=0))
edges = sobel_x + sobel_y

# エッジが多い領域を視線の誘導点として抽出
threshold = np.percentile(edges, 90)
focus_points = np.argwhere(edges > threshold)

# 視線の流れを矢印で描画
# 例：左上→中心→右下
draw.line([(100, 100), (400, 300)], fill="blue", width=3, arrow="last")
draw.line([(400, 300), (600, 500)], fill="blue", width=3, arrow="last")

# アイコンを描画
draw.text((80, 80), "👁️", font_size=50)

img.save("gaze_flow.jpg")
```

### ユーザー体験
```
[画像表示: 青い矢印で視線の流れを示した写真]

👁️ **視線の分析**

「視線は左上から入り、主要被写体に向かい、右下へと流れています。
この流れは自然で良いですが、背景の明るい箇所（黄色丸）が視線を少し邪魔しています。

→ 背景を暗くするか、ボカすと視線がメインに集中します」
```

---

## 5. ヒストグラム解析と可視化（Visual math）

### 概要
写真のヒストグラムを解析し、露出・色分布の改善点を可視化します。

### ユースケース
- **露出の確認**: トーンカーブが適切か
- **色のバランス**: 各チャンネルの分布を確認

### 実装イメージ

```python
import numpy as np
import matplotlib.pyplot as plt
from PIL import Image

img = np.array(Image.open("photo.jpg"))

# RGBヒストグラム
fig, axes = plt.subplots(2, 2, figsize=(10, 8))
axes[0, 0].imshow(img)
axes[0, 0].set_title("Original")
axes[0, 0].axis('off')

# RGBチャンネルごとのヒストグラム
colors = ['red', 'green', 'blue']
for i, color in enumerate(colors):
    axes[0, 1].hist(img[:,:,i].ravel(), bins=256, color=color, alpha=0.5, label=color)

axes[0, 1].set_title("RGB Histogram")
axes[0, 1].legend()
axes[0, 1].set_xlabel("Brightness")
axes[0, 1].set_ylabel("Pixel Count")

# 明度のヒストグラム
luminance = 0.299 * img[:,:,0] + 0.587 * img[:,:,1] + 0.114 * img[:,:,2]
axes[1, 0].hist(luminance.ravel(), bins=256, color='gray')
axes[1, 0].set_title("Luminance Histogram")
axes[1, 0].set_xlabel("Brightness")

# 理想的なヒストグラム（参考）
axes[1, 1].hist(luminance.ravel(), bins=256, color='lightblue', alpha=0.5, label='Actual')
ideal_luminance = np.random.gamma(2, 30, 10000)
axes[1, 1].hist(ideal_luminance, bins=256, color='orange', alpha=0.5, label='Ideal')
axes[1, 1].set_title("Ideal Histogram Comparison")
axes[1, 1].legend()

plt.tight_layout()
plt.savefig("histogram_analysis.png")

# Pythonで解析結果を返す
analysis = {
    "histogram_image": "histogram_analysis.png",
    "findings": [
        "シャドウ部（0-50）にデータ不足 → コントラストが不足",
        "ハイライト部（200-255）に飛びあり → 露出オーバーの可能性",
        "Rチャンネルが他より明るい → ウォームトーン寄り"
    ],
    "recommendations": [
        "露出補正: -0.3EV推奨",
        "コントラスト: +15%",
        "ハイライト: -10%",
        "シャドウ: +20%"
    ]
}
```

### ユーザー体験
```
[画像表示: ヒストグラム解析グラフ]

📊 **ヒストグラム分析**

✅ **良さそうな点:**
- 中間トーンがバランスよく分布しています

⚠️ **改善点:**
- シャドウ部（左端）にデータが不足 → コントラストが物足りない
- ハイライト部（右端）に飛びあり → 一部露出オーバー

💡 **推奨設定:**
- 露出補正: -0.3EV
- コントラスト: +15%
- ハイライト: -10%
- シャドウ: +20%
```

---

## 6. 黄金比スパイラルのオーバーレイ

### 概要
黄金比のフィボナッチスパイラルを写真にオーバーレイし、構図を分析します。

### ユースケース
- **構図の評価**: 主要被写体が黄金比のポイントに配置されているか
- **改善案**: カメラをどの方向に動かせば良くなるか

### 実装イメージ

```python
import numpy as np
from PIL import Image, ImageDraw
import math

def draw_golden_spiral(draw, center, scale, iterations=5):
    a = scale
    b = scale * 0.618
    for i in range(iterations):
        angle = i * 90
        # 四分円を描画
        for theta in np.linspace(angle, angle + 90, 50):
            theta_rad = math.radians(theta)
            r = a * (0.618 ** (angle / 90))
            x = center[0] + r * math.cos(theta_rad)
            y = center[1] + r * math.sin(theta_rad)
            draw.line([(center[0] + (r-1) * math.cos(theta_rad),
                        center[1] + (r-1) * math.sin(theta_rad)),
                       (x, y)], fill="yellow", width=2)
        a *= 0.618
        b *= 0.618

img = Image.open("photo.jpg")
draw = ImageDraw.Draw(img)

# 黄金比のスパイラルを4方向描画
w, h = img.size
centers = [(0, 0), (w, 0), (0, h), (w, h)]
for center in centers:
    draw_golden_spiral(draw, center, min(w, h) * 0.4)

img.save("golden_ratio.jpg")
```

### ユーザー体験
```
[画像表示: 黄金比のスパイラルがオーバーレイされた写真]

🌀 **構図分析（黄金比）**

「主要被写体（黄色丸）は黄金比の交点に近い配置です！これは良い構図です。

ただし、もう少し左に配置すると、スパイラルの流れに沿った動きが出ます。

→ カメラを約15°右に回転すると、より黄金比に近づきます」
```

---

## 7. 複数候補の並列比較

### 概要
1枚の写真から複数の改善候補を生成し、並列比較してベストなアプローチを提案します。

### ユースケース
- **露出の調整**: -0.3EV, 0EV, +0.3EV の3パターン
- **色温度**: クール、ニュートラル、ウォームの3パターン
- **構図**: クロップ位置の違い

### 実装イメージ

```python
# 複数のバリエーションを生成
variations = {
    "exposure": [-0.5, -0.3, 0, +0.3, +0.5],
    "temperature": [4500, 5000, 5500, 6000, 6500],
    "contrast": [0.8, 0.9, 1.0, 1.1, 1.2]
}

# 各パラメータの組み合わせを試してベストを探す
best_score = 0
best_params = {}
for exp in variations["exposure"]:
    for temp in variations["temperature"]:
        for cont in variations["contrast"]:
            # 画像処理
            processed = apply_parameters(img, exp, temp, cont)

            # AIでスコアリング
            score = evaluate_image_quality(processed)

            if score > best_score:
                best_score = score
                best_params = {"exposure": exp, "temperature": temp, "contrast": cont}

# ベストなパラメータで最終画像を生成
best_image = apply_parameters(img, best_params["exposure"],
                              best_params["temperature"],
                              best_params["contrast"])
```

### ユーザー体験
```
[画像表示: 3x3のグリッドで9パターンを並べて比較]

🔍 **パラメータ探索**

「9パターンの比較結果、このパターンがベストです！

【ベスト設定】
- 露出: -0.3EV
- 色温度: 5500K（ニュートラル）
- コントラスト: +10%

【他の候補との比較】
- 露出0EVは少し明るすぎ
- 色温度4500Kは青すぎ
- コントラスト+20%は少し硬くなりすぎ
```

---

## 8. 時系列変化の可視化

### 概要
パラメータを徐々に変化させ、どのタイミングで最適になるかをアニメーションで表示します。

### ユースケース
- **露出の変化**: -1.0EV から +1.0EV まで0.1EV刻み
- **色温度**: 4000K から 7000K まで100K刻み

### 実装イメージ

```python
import matplotlib.pyplot as plt
from PIL import Image, ImageDraw
import numpy as np

img = np.array(Image.open("photo.jpg"))

frames = []
for exposure in np.linspace(-1.0, 1.0, 21):  # -1.0EV to +1.0EV
    # 露出調整
    adjusted = np.clip(img * (2 ** exposure), 0, 255).astype(np.uint8)
    frames.append(Image.fromarray(adjusted))

# GIFアニメーションを作成
frames[0].save("exposure_sweep.gif",
               save_all=True,
               append_images=frames[1:],
               duration=200,
               loop=0)

# 各フレームの品質スコアを計算
scores = [evaluate_quality(frame) for frame in frames]
best_exposure = np.linspace(-1.0, 1.0, 21)[np.argmax(scores)]

# グラフも作成
plt.plot(np.linspace(-1.0, 1.0, 21), scores)
plt.axvline(best_exposure, color='red', linestyle='--', label='Best')
plt.xlabel("Exposure (EV)")
plt.ylabel("Quality Score")
plt.legend()
plt.savefig("exposure_score_curve.png")
```

### ユーザー体験
```
[アニメーション: 露出を変化させたGIF]

[グラフ: 露出 vs 品質スコアの曲線]

📈 **最適露出探索**

「露光を変化させた結果、**-0.3EV** が最適です！

【解析】
- -0.3EV: スコア 92.5 ⭐️ ベスト
- 0.0EV: スコア 85.3
- +0.3EV: スコア 78.1

【参考】
赤い点線が最適な露出値です」
```

---

## 実装アプローチ

### プロンプトの変更

現在の分析プロンプトを以下のように変更することで、Agentic Visionの機能を活用できます。

**変更前**:
```
あなたは写真講評のプロです。次の写真を詳細に評価してください。
採点項目は構図、露出、色彩、ライティング、ピント、現像、距離感、意図の明確さの8項目です。
...
```

**変更後**:
```
あなたは写真講評のプロです。次の写真を詳細に評価してください。

重要: 写真を分析する際、以下の手順に従ってください：

1. **Think**: まず写真全体を観察し、問題点を特定してください。
2. **Act**: Pythonコードを書いて以下の操作を実行してください：
   - 問題のある領域をクロップして拡大表示
   - 改善ポイントを赤い線や丸でアノテーション
   - 視線の流れを青い矢印で可視化
   - ヒストグラムをプロット
3. **Observe**: 生成された画像やグラフを確認し、詳細なアドバイスを作成してください。

採点項目は構図、露出、色彩、ライティング、ピント、現像、距離感、意図の明確さの8項目です。
各項目は0〜10点で採点し、短い講評コメントと具体的な改善提案を必ず記述してください。

出力形式:
```json
{
  "overallScore": 8,
  "analysis": "分析の詳細...",
  "annotations": "アノテーション付き画像のパス",
  "visualizations": ["ヒストグラム.png", "視線の流れ.png"],
  "zoomedRegions": [{"region": "右上の空", "image": "zoomed.jpg"}]
}
```
```

### コードの変更点

#### 1. gemini.go の変更

```go
func (g *GeminiClient) AnalyzeImageWithAgenticVision(ctx context.Context, imageURL string) (*AgenticAnalysisResult, error) {
    if err := g.Ensure(ctx); err != nil {
        return nil, err
    }

    imageData, mimeType, err := fetchImageBytes(ctx, imageURL)
    if err != nil {
        return nil, err
    }

    // Agentic Vision有効化
    analysisPrompt := strings.Join([]string{
        "あなたは写真講評のプロです。Agentic Vision（Code Execution）を使って詳細な分析を行ってください。",
        "1. Think: 写真を観察し、問題点を特定",
        "2. Act: Pythonコードで画像操作（クロップ、アノテーション、ヒストグラムなど）",
        "3. Observe: 結果を確認し、アドバイスを作成",
    }, "\n")

    contents := []*genai.Content{
        genai.NewContentFromParts([]*genai.Part{
            genai.NewPartFromText(analysisPrompt),
            genai.NewPartFromBytes(imageData, mimeType),
        }, genai.RoleUser),
    }

    response, err := g.client.Models.GenerateContent(ctx, "gemini-3-flash-preview", contents, &genai.GenerateContentConfig{
        ResponseMIMEType: "application/json",
        ResponseSchema:   agenticAnalysisResponseSchema(),
        Tools: []*genai.Tool{
            {CodeExecution: &genai.ToolCodeExecution{}},  // ここが重要！
        },
    })
    if err != nil {
        return nil, err
    }

    var result AgenticAnalysisResult
    if err := json.Unmarshal([]byte(response.Text()), &result); err != nil {
        return nil, err
    }
    return &result, nil
}
```

#### 2. 新しいレスポンス構造

```go
type AgenticAnalysisResult struct {
    OverallScore int
    Analysis    string
    Annotations  string   // アノテーション付き画像のパス
    Visualizations []string  // ヒストグラムなどのパス
    ZoomedRegions []ZoomedRegion
    GazeFlow     string   // 視線の流れ画像
    Histogram    string   // ヒストグラム画像
}

type ZoomedRegion struct {
    Region string
    Image  string
    Advice string
}
```

---

## パフォーマンス考慮

Agentic Visionは複数回のAPI呼び出しとコード実行を伴うため、以下の考慮が必要です：

1. **遅延**: 従来の2-3倍の時間がかかる可能性
   - 対策: 非同期処理、プログレスバー表示

2. **コスト**: コード実行による追加コスト
   - 対策: キャッシュ、不要な分析のスキップ

3. **画像サイズ**: 高解像度画像は処理に時間がかかる
   - 対策: 分析用に事前にリサイズ

---

## フロントエンドの変更

### 分析結果の表示

```
┌─────────────────────────────────────────┐
│ 📸 写真分析結果                           │
├─────────────────────────────────────────┤
│ [元写真]           [アノテーション版]   │
│                                          │
│ [視線の流れ]      [ヒストグラム]        │
│                                          │
│ 🔍 拡大分析                              │
│ ┌─────────────┐  ┌─────────────┐       │
│ │[拡大1]      │  │[拡大2]      │       │
│ │白飛び箇所   │  │構図の問題   │       │
│ └─────────────┘  └─────────────┘       │
│                                          │
│ 📊 スコア: 8/10                          │
│                                          │
│ 💡 アドバイス:                           │
│ ...                                      │
└─────────────────────────────────────────┘
```

---

## まとめ

Agentic Visionを活用することで、以下の機能を実現できます：

| 機能 | 説明 | 難易度 | インパクト |
|------|------|--------|-----------|
| 自動ズーム＆詳細分析 | 問題箇所を検出して拡大 | ⭐️⭐️ | 🔥🔥🔥 |
| 赤ペンアノテーション | 改善ポイントを直接書き込み | ⭐️ | 🔥🔥🔥🔥🔥 |
| Before/After比較 | 差異をヒートマップで可視化 | ⭐️⭐️ | 🔥🔥🔥 |
| 視線の流れの可視化 | 視線の動きを矢印で表示 | ⭐️⭐️⭐️ | 🔥🔥 |
| ヒストグラム解析 | トーンカーブを分析 | ⭐️ | 🔥🔥 |
| 黄金比スパイラル | 構図を分析 | ⭐️ | 🔥🔥 |
| 複数候補の比較 | パラメータ探索 | ⭐️⭐️⭐️⭐️ | 🔥🔥🔥 |
| 時系列変化の可視化 | アニメーション表示 | ⭐️⭐️⭐️⭐️ | 🔥🔥 |

ハッカソン用としては、「赤ペンアノテーション」と「自動ズーム分析」から始めるのがオススメです。これらは視覚的に分かりやすく、インパクトも大きいです！

---

## 参考リンク

- [Agentic Vision in Gemini 3 Flash](https://blog.google/innovation-and-ai/technology/developers-tools/agentic-vision-gemini-3-flash/)
- [Google AI Studio Demo](https://aistudio.google.com/apps/bundled/gemini_visual_thinking)
- [Code Execution Docs](https://ai.google.dev/gemini-api/docs/code-execution#images)
