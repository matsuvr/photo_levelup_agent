"use client";

import { useCallback, useState } from "react";
import type { Session } from "@/lib/sessions";

type SessionListProps = {
	sessions: Session[];
	currentSessionId?: string;
	onSelectSession: (session: Session) => void;
	onNewSession: () => void;
	loading?: boolean;
};

// Helper to check if a URL is valid for img src
function isValidImageUrl(url: string | undefined): url is string {
	if (!url) return false;
	// gs:// URLs cannot be loaded directly by browsers
	if (url.startsWith("gs://")) return false;
	// Accept http://, https://, data:, blob: URLs
	return (
		url.startsWith("https://") ||
		url.startsWith("http://") ||
		url.startsWith("data:") ||
		url.startsWith("blob:")
	);
}

// Thumbnail component with error handling
function SessionThumbnail({ src, alt }: { src: string; alt: string }) {
	const [hasError, setHasError] = useState(false);

	const handleError = useCallback(() => {
		setHasError(true);
	}, []);

	if (hasError) {
		return (
			<div className="session-item-thumbnail session-item-thumbnail-fallback">
				<svg
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					strokeWidth="1.5"
					aria-hidden="true"
				>
					<rect x="3" y="3" width="18" height="18" rx="2" />
					<circle cx="8.5" cy="8.5" r="1.5" />
					<path d="M21 15l-5-5L5 21" />
				</svg>
			</div>
		);
	}

	return (
		<div className="session-item-thumbnail">
			<img src={src} alt={alt} onError={handleError} />
		</div>
	);
}

export function SessionList({
	sessions,
	currentSessionId,
	onSelectSession,
	onNewSession,
	loading = false,
}: SessionListProps) {
	if (loading) {
		return (
			<div className="session-list-loading">
				<div className="loading-spinner" />
				<span>読み込み中...</span>
			</div>
		);
	}

	return (
		<div className="session-list">
			{/* New Session Button */}
			<button
				type="button"
				className="session-new-button"
				onClick={onNewSession}
			>
				<svg
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					strokeWidth="2"
					aria-hidden="true"
				>
					<path d="M12 5v14M5 12h14" />
				</svg>
				新しいセッション
			</button>

			{/* Session Items */}
			{sessions.length === 0 ? (
				<div className="session-list-empty">
					<p>まだセッションがありません</p>
					<p className="session-list-empty-hint">
						写真をアップロードして分析を始めましょう
					</p>
				</div>
			) : (
				<ul className="session-items">
					{sessions.map((session) => (
						<li key={session.id}>
							<button
								type="button"
								className={`session-item ${session.id === currentSessionId ? "active" : ""}`}
								onClick={() => onSelectSession(session)}
							>
								{(() => {
									const imageUrl = session.originalPhotoUrl || session.photoUrl;
									return isValidImageUrl(imageUrl) ? (
										<SessionThumbnail src={imageUrl} alt="" />
									) : null;
								})()}
								<div className="session-item-content">
									<div className="session-item-main">
										<span className="session-item-title">{session.title}</span>
										{session.overallScore !== undefined && (
											<span className="session-item-score">
												{session.overallScore}/10
											</span>
										)}
									</div>
									<div className="session-item-meta">
										<span className="session-item-messages">
											{session.messageCount ?? session.messages.length}{" "}
											メッセージ
										</span>
										<span className="session-item-date">
											{formatDate(session.updatedAt.toDate())}
										</span>
									</div>
								</div>
							</button>
						</li>
					))}
				</ul>
			)}
		</div>
	);
}

function formatDate(date: Date): string {
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

	if (diffDays === 0) {
		return "今日";
	}
	if (diffDays === 1) {
		return "昨日";
	}
	if (diffDays < 7) {
		return `${diffDays}日前`;
	}

	return date.toLocaleDateString("ja-JP", {
		month: "short",
		day: "numeric",
	});
}
