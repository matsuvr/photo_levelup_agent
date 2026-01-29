import "./globals.css"
import type { Metadata } from "next"

export const metadata: Metadata = {
  title: "Photo Levelup Agent",
  description: "写真の分析と改善アドバイスを提供するAIコーチ",
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="ja">
      <body>{children}</body>
    </html>
  )
}
