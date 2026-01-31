"use client"

import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { useCallback, useEffect, useMemo, useRef, useState, CSSProperties } from "react"
import { useAuth } from "@/context/AuthContext"
import { secureLog } from "@/lib/secure-log"
import {
  type Session,
  type ChatMessage,
  type AnalysisResult,
  type CategoryScore,
  getUserSessions,
  createSession,
  addMessageToSession,
  generateSessionId,
  updateSessionMetadata,
} from "@/lib/sessions"
import { Timestamp } from "firebase/firestore"
import { SlideMenu } from "@/app/components/SlideMenu"
import { SessionList } from "@/app/components/SessionList"

// Re-export types for backward compatibility within component
type PhotoSession = {
  originalPreview: string
  enhancedPreview: string
  analysis: AnalysisResult
}

const initialMessage: ChatMessage = {
  id: "welcome",
  role: "agent",
  content:
    "**写真をアップロードしてください。** 採点と改善提案を行い、その結果をもとに理想的な写真を生成します。",
  timestamp: Timestamp.now(),
}

export default function Home() {
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const messagesEndRef = useRef<HTMLDivElement | null>(null)

  // State
  const [currentSessionId, setCurrentSessionId] = useState<string>("")
  const [isUploading, setIsUploading] = useState(false)
  const [isChatting, setIsChatting] = useState(false)
  const [photoSession, setPhotoSession] = useState<PhotoSession | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([initialMessage])
  const [draft, setDraft] = useState("")
  const [showBottomSheet, setShowBottomSheet] = useState(false)
  const [showAnalysisPopup, setShowAnalysisPopup] = useState(false)
  const [showSlideMenu, setShowSlideMenu] = useState(false)
  const [activeTab, setActiveTab] = useState<"photo" | "analysis">("photo")

  // Session Management State
  const [userSessions, setUserSessions] = useState<Session[]>([])
  const [loadingSessions, setLoadingSessions] = useState(false)

  const { user, loading: authLoading, signInWithGoogle, signOut } = useAuth()

  const canChat = useMemo(() => Boolean(photoSession), [photoSession])

  // Initialize session ID on mount
  useEffect(() => {
    if (!currentSessionId) {
      setCurrentSessionId(generateSessionId())
    }
  }, [currentSessionId])

  // Load user sessions when menu opens or user logs in
  useEffect(() => {
    if (user && showSlideMenu) {
      loadSessions()
    }
  }, [user, showSlideMenu])

  const loadSessions = async () => {
    if (!user) return
    setLoadingSessions(true)
    try {
      const sessions = await getUserSessions(user.uid)
      setUserSessions(sessions)
    } catch (error) {
      secureLog.error("Failed to load sessions:", error)
    } finally {
      setLoadingSessions(false)
    }
  }

  const handleNewSession = () => {
    setCurrentSessionId(generateSessionId())
    setMessages([initialMessage])
    setPhotoSession(null)
    setShowSlideMenu(false)
  }

  const handleSelectSession = (session: Session) => {
    setCurrentSessionId(session.id)
    setMessages(session.messages)

    // Construct photoSession from session data if available
    if (session.photoUrl && session.overallScore !== undefined) {
      // Note: In a real app we would store full analysis details in Firestore
      // For now we recover basic state. If analysis details are missing from session type,
      // we might need to fetch them or accept partial state.
      // Since our Session type stores full messages, we can check if any message has cards
      const lastAnalysisMsg = [...session.messages].reverse().find(m => m.analysisCard)
      const lastPhotoMsg = [...session.messages].reverse().find(m => m.photoCard)

      if (lastAnalysisMsg?.analysisCard && lastPhotoMsg?.photoCard) {
        setPhotoSession({
          originalPreview: lastPhotoMsg.photoCard.original,
          enhancedPreview: lastPhotoMsg.photoCard.enhanced,
          analysis: lastAnalysisMsg.analysisCard
        })
      } else {
        setPhotoSession(null)
      }
    } else {
      setPhotoSession(null)
    }

    setShowSlideMenu(false)
  }

  const chartItems = useMemo(() => {
    if (!photoSession) {
      return []
    }

    const analysis = photoSession.analysis
    return [
      { label: "構図", value: analysis.composition.score },
      { label: "露出", value: analysis.exposure.score },
      { label: "色彩", value: analysis.color.score },
      { label: "ライティング", value: analysis.lighting.score },
      { label: "ピント", value: analysis.focus.score },
      { label: "現像", value: analysis.development.score },
      { label: "距離感", value: analysis.distance.score },
      { label: "意図", value: analysis.intentClarity.score },
    ]
  }, [photoSession])

  const categoryRows = useMemo(() => {
    if (!photoSession) {
      return []
    }

    const analysis = photoSession.analysis
    return [
      { label: "構図", data: analysis.composition },
      { label: "露出", data: analysis.exposure },
      { label: "色彩", data: analysis.color },
      { label: "ライティング", data: analysis.lighting },
      { label: "ピント", data: analysis.focus },
      { label: "現像", data: analysis.development },
      { label: "距離感", data: analysis.distance },
      { label: "意図の明確さ", data: analysis.intentClarity },
    ]
  }, [photoSession])

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }

  const handlePickFile = () => {
    fileInputRef.current?.click()
    setShowBottomSheet(false)
  }

  const handleFileChange = async (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    const file = event.target.files?.[0]
    if (!file) {
      return
    }

    if (!file.type.startsWith("image/")) {
      addLocalMessage({
        role: "agent",
        content: "**エラー:** 画像ファイルを選択してください。",
      })
      return
    }

    setIsUploading(true)

    try {
      const originalPreview = URL.createObjectURL(file)
      const formData = new FormData()
      formData.append("image", file)
      formData.append("sessionId", currentSessionId)

      addLocalMessage({
        role: "agent",
        content: "📷 写真を受け取りました。分析中...",
      })

      // Submit for async processing
      const submitResponse = await fetch("/api/analyze", {
        method: "POST",
        body: formData,
      })

      if (!submitResponse.ok) {
        const payload = await submitResponse.json().catch(() => null)
        const message = payload?.error ?? "アップロードに失敗しました。"
        addLocalMessage({ role: "agent", content: `**エラー:** ${message}` })
        return
      }

      const submitData = (await submitResponse.json()) as {
        jobId: string
        status: string
      }

      secureLog.info("Job submitted:", submitData.jobId)

      // Poll for job completion
      const pollInterval = 2000 // 2 seconds
      const maxAttempts = 120 // 4 minutes max
      let attempts = 0

      const pollForResult = async (): Promise<{
        enhancedImageUrl: string
        analysis: AnalysisResult
        initialAdvice: string
      } | null> => {
        while (attempts < maxAttempts) {
          attempts++
          await new Promise(resolve => setTimeout(resolve, pollInterval))

          const statusResponse = await fetch(
            `/api/analyze/status?jobId=${encodeURIComponent(submitData.jobId)}`
          )

          if (!statusResponse.ok) {
            secureLog.error("Status check failed:", statusResponse.status)
            continue
          }

          const statusData = await statusResponse.json()
          secureLog.info("Job status:", statusData.status)

          if (statusData.status === "completed" && statusData.result) {
            return statusData.result
          }

          if (statusData.status === "failed") {
            throw new Error(statusData.error || "分析に失敗しました")
          }

          // Still processing, continue polling
        }

        throw new Error("分析がタイムアウトしました。もう一度お試しください。")
      }

      const data = await pollForResult()
      if (!data) {
        throw new Error("分析結果を取得できませんでした")
      }

      const sessionData: PhotoSession = {
        originalPreview,
        enhancedPreview: data.enhancedImageUrl,
        analysis: data.analysis,
      }

      setPhotoSession(sessionData)

      // Add message with embedded photo card and analysis card
      const newMessage: ChatMessage = {
        id: `${Date.now()}-${Math.random()}`,
        role: "agent",
        content: data.initialAdvice,
        timestamp: Timestamp.now(),
        photoCard: { original: originalPreview, enhanced: data.enhancedImageUrl },
        analysisCard: data.analysis,
      }

      // Update local state
      setMessages(prev => [...prev, newMessage])
      setTimeout(scrollToBottom, 50)

      // Sync with Firestore if logged in
      if (user) {
        try {
          await createSession(user.uid, currentSessionId, newMessage)
          await updateSessionMetadata(currentSessionId, {
            overallScore: data.analysis.overallScore,
            photoUrl: data.enhancedImageUrl
          })
        } catch (e) {
          secureLog.error("Error saving session:", e)
        }
      }

    } catch (error) {
      secureLog.error(error)
      const errorMessage = error instanceof Error ? error.message : "通信中にエラーが発生しました。"
      addLocalMessage({
        role: "agent",
        content: `**エラー:** ${errorMessage}`,
      })
    } finally {
      setIsUploading(false)
    }
  }


  // Helper for adding messages only locally (for errors, loading status)
  const addLocalMessage = (message: Omit<ChatMessage, "id" | "timestamp">) => {
    setMessages((prev) => [
      ...prev,
      {
        ...message,
        id: `${Date.now()}-${Math.random()}`,
        timestamp: Timestamp.now()
      },
    ])
    setTimeout(scrollToBottom, 50)
  }

  const handleSubmit = async () => {
    const text = draft.trim()
    if (!text) { // Allow chatting without photo session if needed, but UI disables it
      return
    }

    const userMessage: ChatMessage = {
      id: `${Date.now()}-${Math.random()}`,
      role: "user",
      content: text,
      timestamp: Timestamp.now()
    }

    setMessages(prev => [...prev, userMessage])
    setDraft("")
    setIsChatting(true)
    setTimeout(scrollToBottom, 50)

    // Sync user message to Firestore
    if (user) {
      addMessageToSession(currentSessionId, userMessage).catch(secureLog.error)
    }

    try {
      const response = await fetch("/api/chat", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          sessionId: currentSessionId,
          message: text,
        }),
      })

      if (!response.ok) {
        const payload = await response.json().catch(() => null)
        const message = payload?.error ?? "チャットの送信に失敗しました。"
        addLocalMessage({ role: "agent", content: `**エラー:** ${message}` })
        return
      }

      const data = (await response.json()) as { reply: string }

      const agentMessage: ChatMessage = {
        id: `${Date.now()}-${Math.random()}`,
        role: "agent",
        content: data.reply,
        timestamp: Timestamp.now()
      }

      setMessages(prev => [...prev, agentMessage])
      setTimeout(scrollToBottom, 50)

      // Sync agent message to Firestore
      if (user) {
        addMessageToSession(currentSessionId, agentMessage).catch(secureLog.error)
      }

    } catch (error) {
      secureLog.error(error)
      addLocalMessage({
        role: "agent",
        content: "**エラー:** 通信中にエラーが発生しました。",
      })
    } finally {
      setIsChatting(false)
    }
  }

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault()
      handleSubmit()
    }
  }

  return (
    <div className="page">
      <SlideMenu isOpen={showSlideMenu} onClose={() => setShowSlideMenu(false)}>
        <SessionList
          sessions={userSessions}
          currentSessionId={currentSessionId}
          onSelectSession={handleSelectSession}
          onNewSession={handleNewSession}
          loading={loadingSessions}
        />
      </SlideMenu>

      <header className="header">
        <div className="header-left" style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          {user && (
            <button
              type="button"
              className="my-page-button"
              onClick={() => setShowSlideMenu(true)}
              aria-label="マイページを開く"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M3 12h18M3 6h18M3 18h18" />
              </svg>
              マイページ
            </button>
          )}
          <div>
            <h1>Photo Levelup Agent</h1>
            <p>写真を分析し、コンテスト受賞レベルへ導くAIコーチ</p>
          </div>
        </div>
        <div className="header-right">
          <span className="badge">Gemini 3</span>
          {!authLoading && (
            user ? (
              <div className="auth-status">
                <span className="user-email">{user.email} でログイン中</span>
                <button type="button" className="auth-button logout" onClick={signOut}>
                  ログアウト
                </button>
              </div>
            ) : (
              <button type="button" className="auth-button login" onClick={signInWithGoogle}>
                Googleでログイン
              </button>
            )
          )}
        </div>
      </header>

      <main className="main">
        {/* Mobile Layout */}
        <div className="mobile-layout">
          {/* Swipeable Tabs Section */}
          {photoSession && (
            <div className="tabs-section">
              <div className="tabs-header">
                <button
                  type="button"
                  className={`tab-button ${activeTab === "photo" ? "active" : ""}`}
                  onClick={() => setActiveTab("photo")}
                >
                  📸 写真
                </button>
                <button
                  type="button"
                  className={`tab-button ${activeTab === "analysis" ? "active" : ""}`}
                  onClick={() => setActiveTab("analysis")}
                >
                  📊 分析
                </button>
              </div>
              <SwipeableTabs activeTab={activeTab} onTabChange={setActiveTab}>
                <div className="tab-content">
                  {activeTab === "photo" ? (
                    <BeforeAfterSlider
                      beforeSrc={photoSession.originalPreview}
                      afterSrc={photoSession.enhancedPreview}
                    />
                  ) : (
                    <div className="analysis-preview-card" onClick={() => setShowAnalysisPopup(true)}>
                      <div className="analysis-score-big">
                        <span className="score-number">{photoSession.analysis.overallScore}</span>
                        <span className="score-max">/ 10</span>
                      </div>
                      <RadarChart items={chartItems} />
                      <button type="button" className="details-button">
                        詳細を見る
                      </button>
                    </div>
                  )}
                </div>
              </SwipeableTabs>
            </div>
          )}

          {/* Chat Panel */}
          <div className="chat-panel">
            <div className="chat-messages">
              {messages.map((message) => (
                <div key={message.id} className={`message ${message.role}`}>
                  {message.role === "agent" ? (
                    <>
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>
                        {message.content}
                      </ReactMarkdown>
                      {message.photoCard && (
                        <div className="embedded-photo-card">
                          <BeforeAfterSlider
                            beforeSrc={message.photoCard.original}
                            afterSrc={message.photoCard.enhanced}
                            compact
                          />
                        </div>
                      )}
                      {message.analysisCard && (
                        <div
                          className="embedded-analysis-card"
                          onClick={() => {
                            if (photoSession) setShowAnalysisPopup(true)
                          }}
                        >
                          <div className="mini-score">
                            <span className="mini-score-value">{message.analysisCard.overallScore}</span>
                            <span className="mini-score-label">/ 10</span>
                          </div>
                          <span className="tap-hint">タップで詳細</span>
                        </div>
                      )}
                    </>
                  ) : (
                    message.content
                  )}
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>

            {/* Composer */}
            <div className="composer">
              <button
                type="button"
                className="camera-button"
                onClick={() => setShowBottomSheet(true)}
                disabled={isUploading}
                aria-label="写真をアップロード"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z" />
                  <circle cx="12" cy="13" r="4" />
                </svg>
              </button>
              <textarea
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={
                  canChat
                    ? "質問を入力... (Enter で送信)"
                    : "まず写真をアップロードしてください"
                }
                disabled={!canChat || isChatting}
                rows={1}
              />
              <button
                className="send-button"
                type="button"
                onClick={handleSubmit}
                disabled={!canChat || isChatting || !draft.trim()}
              >
                {isChatting ? "..." : "送信"}
              </button>
            </div>
          </div>
        </div>

        {/* Desktop Sidebar (hidden on mobile) */}
        <aside className="sidebar">
          {/* Upload Section */}
          <div className="sidebar-section">
            <h2>写真アップロード</h2>
            <div className="dropzone">
              <p>写真を選択して分析を開始</p>
              <button
                className="button"
                type="button"
                onClick={handlePickFile}
                disabled={isUploading}
              >
                {isUploading ? "送信中..." : "写真を選択"}
              </button>
              <span className="footer-note">JPEG/PNG 推奨 • 最大 20MB</span>
            </div>
          </div>

          {/* Photo Comparison */}
          {photoSession && (
            <div className="sidebar-section">
              <h2>写真比較</h2>
              <div className="photo-grid">
                <div className="photo-item">
                  <span className="photo-label">元</span>
                  <div className="preview">
                    <img src={photoSession.originalPreview} alt="Original" />
                  </div>
                </div>
                <div className="photo-item">
                  <span className="photo-label">生成</span>
                  <div className="preview">
                    <img src={photoSession.enhancedPreview} alt="Enhanced" />
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Analysis Results */}
          {photoSession && (
            <div className="sidebar-section">
              <h2>分析結果</h2>
              <div className="score-overview">
                <span className="score-label">総合スコア</span>
                <span className="score-value">
                  {photoSession.analysis.overallScore}
                </span>
                <span className="score-label">/ 10</span>
              </div>
              <RadarChart items={chartItems} />
              <div className="category-list">
                {categoryRows.map((row) => (
                  <div key={row.label} className="category-row">
                    <div className="category-header">
                      <span>{row.label}</span>
                      <span className="category-score">
                        {row.data.score} / 10
                      </span>
                    </div>
                    <p>{row.data.comment}</p>
                    <p className="category-improvement">
                      💡 {row.data.improvement}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          )}
        </aside>
      </main>

      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        accept="image/*"
        onChange={handleFileChange}
        style={{ display: "none" }}
      />

      {/* Bottom Sheet */}
      <BottomSheet isOpen={showBottomSheet} onClose={() => setShowBottomSheet(false)}>
        <div className="bottom-sheet-content">
          <h3>写真を追加</h3>
          <button type="button" className="sheet-option" onClick={handlePickFile}>
            <span className="option-icon">🖼️</span>
            <span>ライブラリから選択</span>
          </button>
          <button
            type="button"
            className="sheet-option"
            onClick={() => {
              fileInputRef.current?.click()
              setShowBottomSheet(false)
            }}
          >
            <span className="option-icon">📷</span>
            <span>カメラで撮影</span>
          </button>
          <button
            type="button"
            className="sheet-cancel"
            onClick={() => setShowBottomSheet(false)}
          >
            キャンセル
          </button>
        </div>
      </BottomSheet>

      {/* Analysis Popup */}
      {photoSession && (
        <AnalysisPopup
          isOpen={showAnalysisPopup}
          onClose={() => setShowAnalysisPopup(false)}
          analysis={photoSession.analysis}
          chartItems={chartItems}
          categoryRows={categoryRows}
        />
      )}
    </div>
  )
}

// ============ Components ============

type RadarChartItem = {
  label: string
  value: number
}

function RadarChart({ items }: { items: RadarChartItem[] }) {
  if (items.length === 0) {
    return null
  }

  const size = 200
  const padding = 30
  const center = size / 2
  const radius = center - padding
  const angleStep = (Math.PI * 2) / items.length
  const levels = 5

  const points = items
    .map((item, index) => {
      const angle = angleStep * index - Math.PI / 2
      const valueRatio = Math.max(0, Math.min(10, item.value)) / 10
      const r = radius * valueRatio
      const x = center + r * Math.cos(angle)
      const y = center + r * Math.sin(angle)
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(" ")

  const gridRadii = Array.from({ length: levels }, (_, index) => {
    return radius * ((index + 1) / levels)
  })

  return (
    <svg
      className="radar-chart"
      viewBox={`0 0 ${size} ${size}`}
      role="img"
      aria-label="8項目のレーダーチャート"
    >
      {gridRadii.map((gridRadius) => (
        <circle
          key={`grid-${gridRadius}`}
          cx={center}
          cy={center}
          r={gridRadius}
          className="radar-grid"
        />
      ))}
      {items.map((item, index) => {
        const angle = angleStep * index - Math.PI / 2
        const x = center + radius * Math.cos(angle)
        const y = center + radius * Math.sin(angle)
        return (
          <line
            key={`axis-${item.label}`}
            x1={center}
            y1={center}
            x2={x}
            y2={y}
            className="radar-axis"
          />
        )
      })}
      <polygon points={points} className="radar-area" />
      {items.map((item, index) => {
        const angle = angleStep * index - Math.PI / 2
        const labelRadius = radius + 14
        const x = center + labelRadius * Math.cos(angle)
        const y = center + labelRadius * Math.sin(angle)
        return (
          <text
            key={`label-${item.label}`}
            x={x}
            y={y}
            className="radar-label"
            textAnchor={
              x < center - 8 ? "end" : x > center + 8 ? "start" : "middle"
            }
            dominantBaseline={
              y < center - 8 ? "auto" : y > center + 8 ? "hanging" : "middle"
            }
          >
            {item.label}
          </text>
        )
      })}
    </svg>
  )
}

// Bottom Sheet Component
function BottomSheet({
  isOpen,
  onClose,
  children,
}: {
  isOpen: boolean
  onClose: () => void
  children: React.ReactNode
}) {
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden"
    } else {
      document.body.style.overflow = ""
    }
    return () => {
      document.body.style.overflow = ""
    }
  }, [isOpen])

  if (!isOpen) return null

  return (
    <div className="bottom-sheet-overlay" onClick={onClose}>
      <div
        className="bottom-sheet"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="bottom-sheet-handle" />
        {children}
      </div>
    </div>
  )
}

// Before/After Slider Component
function BeforeAfterSlider({
  beforeSrc,
  afterSrc,
  compact = false,
}: {
  beforeSrc: string
  afterSrc: string
  compact?: boolean
}) {
  const [sliderPosition, setSliderPosition] = useState(50)
  const [aspectRatio, setAspectRatio] = useState<string | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const isDragging = useRef(false)

  useEffect(() => {
    let isCancelled = false

    const loadImageSize = (src: string) =>
      new Promise<{ width: number; height: number }>((resolve, reject) => {
        const img = new Image()
        img.onload = () =>
          resolve({
            width: img.naturalWidth,
            height: img.naturalHeight,
          })
        img.onerror = () => reject(new Error("failed to load image"))
        img.src = src
      })

    const updateAspect = async () => {
      try {
        const [beforeSize, afterSize] = await Promise.all([
          loadImageSize(beforeSrc),
          loadImageSize(afterSrc),
        ])

        if (isCancelled) return

        const fallbackWidth = afterSize.width || beforeSize.width
        const fallbackHeight = afterSize.height || beforeSize.height
        if (!fallbackWidth || !fallbackHeight) {
          setAspectRatio(null)
          return
        }

        const ratio = fallbackWidth / fallbackHeight
        setAspectRatio(`${ratio}`)
      } catch (error) {
        secureLog.warn("Failed to read image dimensions", error)
        if (!isCancelled) {
          setAspectRatio(null)
        }
      }
    }

    updateAspect()

    return () => {
      isCancelled = true
    }
  }, [beforeSrc, afterSrc])

  const handleMove = useCallback((clientX: number) => {
    if (!containerRef.current) return
    const rect = containerRef.current.getBoundingClientRect()
    const x = clientX - rect.left
    const percentage = Math.max(0, Math.min(100, (x / rect.width) * 100))
    setSliderPosition(percentage)
  }, [])

  const handleMouseDown = () => {
    isDragging.current = true
  }

  const handleMouseUp = () => {
    isDragging.current = false
  }

  const handleMouseMove = (e: React.MouseEvent) => {
    if (isDragging.current) {
      handleMove(e.clientX)
    }
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    handleMove(e.touches[0].clientX)
  }

  return (
    <div
      ref={containerRef}
      className={`before-after-slider ${compact ? "compact" : ""}`}
      onMouseMove={handleMouseMove}
      onMouseUp={handleMouseUp}
      onMouseLeave={handleMouseUp}
      onTouchMove={handleTouchMove}
      style={aspectRatio ? ({ "--slider-aspect": aspectRatio } as CSSProperties) : undefined}
    >
      <div className="slider-image-container">
        <img src={beforeSrc} alt="元画像" className="slider-image before" />
        <div
          className="slider-image-clip"
          style={{ clipPath: `inset(0 0 0 ${sliderPosition}%)` }}
        >
          <img src={afterSrc} alt="修正後" className="slider-image after" />
        </div>
      </div>
      <div
        className="slider-handle"
        style={{ left: `${sliderPosition}%` }}
        onMouseDown={handleMouseDown}
        onTouchStart={handleMouseDown}
      >
        <div className="slider-handle-bar" />
        <div className="slider-handle-circle">
          <span>◀▶</span>
        </div>
      </div>
      <div className="slider-labels">
        <span className="label-before">元</span>
        <span className="label-after">修正</span>
      </div>
    </div>
  )
}

// Swipeable Tabs Component
function SwipeableTabs({
  activeTab,
  onTabChange,
  children,
}: {
  activeTab: "photo" | "analysis"
  onTabChange: (tab: "photo" | "analysis") => void
  children: React.ReactNode
}) {
  const startX = useRef(0)
  const currentX = useRef(0)

  const handleTouchStart = (e: React.TouchEvent) => {
    startX.current = e.touches[0].clientX
    currentX.current = e.touches[0].clientX
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    currentX.current = e.touches[0].clientX
  }

  const handleTouchEnd = () => {
    const diff = startX.current - currentX.current
    if (Math.abs(diff) > 50) {
      if (diff > 0 && activeTab === "photo") {
        onTabChange("analysis")
      } else if (diff < 0 && activeTab === "analysis") {
        onTabChange("photo")
      }
    }
  }

  return (
    <div
      className="swipeable-tabs"
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
    >
      {children}
    </div>
  )
}

// Analysis Popup Component
function AnalysisPopup({
  isOpen,
  onClose,
  analysis,
  chartItems,
  categoryRows,
}: {
  isOpen: boolean
  onClose: () => void
  analysis: AnalysisResult
  chartItems: RadarChartItem[]
  categoryRows: { label: string; data: CategoryScore }[]
}) {
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden"
    } else {
      document.body.style.overflow = ""
    }
    return () => {
      document.body.style.overflow = ""
    }
  }, [isOpen])

  if (!isOpen) return null

  return (
    <div className="analysis-popup-overlay" onClick={onClose}>
      <div className="analysis-popup" onClick={(e) => e.stopPropagation()}>
        <button type="button" className="popup-close" onClick={onClose}>
          ✕
        </button>
        <h2>分析結果</h2>

        <div className="popup-score-section">
          <div className="popup-score">
            <span className="popup-score-value">{analysis.overallScore}</span>
            <span className="popup-score-label">/ 10</span>
          </div>
          <RadarChart items={chartItems} />
        </div>

        {analysis.summary && (
          <div className="popup-summary">
            <h3>サマリー</h3>
            <p>{analysis.summary}</p>
          </div>
        )}

        {analysis.overallComment && (
          <div className="popup-comment">
            <h3>総評</h3>
            <p>{analysis.overallComment}</p>
          </div>
        )}

        <div className="popup-categories">
          <h3>各項目の詳細</h3>
          {categoryRows.map((row) => (
            <div key={row.label} className="popup-category-row">
              <div className="popup-category-header">
                <span>{row.label}</span>
                <span className="popup-category-score">{row.data.score} / 10</span>
              </div>
              <p className="popup-category-comment">{row.data.comment}</p>
              <p className="popup-category-improvement">💡 {row.data.improvement}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
