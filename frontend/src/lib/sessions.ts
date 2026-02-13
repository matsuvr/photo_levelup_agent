"use client";

import {
	collection,
	type DocumentData,
	doc,
	getDoc,
	getDocs,
	orderBy,
	query,
	setDoc,
	Timestamp,
	updateDoc,
	where,
} from "firebase/firestore";
import { db } from "./firebase";

export type ChatMessage = {
	id: string;
	role: "user" | "agent";
	content: string;
	timestamp: Timestamp;
	analysisCard?: AnalysisResult;
	photoCard?: { original?: string; enhanced?: string; cleanEnhanced?: string };
};

export type AnalysisResult = {
	photoSummary: string;
	summary: string;
	overallComment: string;
	overallScore: number;
	composition: CategoryScore;
	exposure: CategoryScore;
	color: CategoryScore;
	lighting: CategoryScore;
	focus: CategoryScore;
	development: CategoryScore;
	distance: CategoryScore;
	intentClarity: CategoryScore;
};

export type CategoryScore = {
	score: number;
	comment: string;
	improvement: string;
};

export type Session = {
	id: string;
	userId: string;
	createdAt: Timestamp;
	updatedAt: Timestamp;
	title: string;
	overallScore?: number;
	photoUrl?: string;
	originalPhotoUrl?: string;
	cleanEnhancedPhotoUrl?: string;
	messages: ChatMessage[];
	messageCount?: number;
};

const SESSIONS_COLLECTION = "sessions";

// Convert backend image URLs to frontend proxy URLs (/api/image?object=...)
export function toFrontendImageUrl(
	url: string | undefined,
): string | undefined {
	if (!url) return undefined;
	if (url.startsWith("/api/image?")) return url;
	if (url.startsWith("blob:") || url.startsWith("data:")) return url;

	// gs://bucket/objectname → /api/image?object=objectname
	if (url.startsWith("gs://")) {
		const parts = url.split("/");
		if (parts.length >= 4) {
			const objectName = parts.slice(3).join("/");
			return `/api/image?object=${encodeURIComponent(objectName)}`;
		}
		console.error(`Malformed gs:// URL: ${url}`);
		return undefined;
	}

	// https://backend/photo/image?object=xxx → /api/image?object=xxx
	if (url.startsWith("https://") || url.startsWith("http://")) {
		try {
			const parsed = new URL(url);
			const objectParam = parsed.searchParams.get("object");
			if (objectParam) {
				return `/api/image?object=${encodeURIComponent(objectParam)}`;
			}
		} catch {
			/* invalid URL */
		}
		return url;
	}

	return undefined;
}

// Recursively remove undefined values from an object for Firestore compatibility
function sanitizeForFirestore<T>(obj: T): T {
	if (obj === null || obj === undefined) {
		return obj;
	}
	if (Array.isArray(obj)) {
		return obj.map((item) => sanitizeForFirestore(item)) as T;
	}
	if (typeof obj === "object" && !(obj instanceof Timestamp)) {
		const result: Record<string, unknown> = {};
		for (const [key, value] of Object.entries(obj)) {
			if (value !== undefined) {
				result[key] = sanitizeForFirestore(value);
			}
		}
		return result as T;
	}
	return obj;
}

// Generate a unique session ID
export function generateSessionId(): string {
	return `session-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

// Create a new session
export async function createSession(
	userId: string,
	sessionId: string,
	initialMessage?: ChatMessage,
): Promise<Session> {
	const now = Timestamp.now();
	const session: Session = {
		id: sessionId,
		userId,
		createdAt: now,
		updatedAt: now,
		title: formatSessionTitle(now.toDate()),
		messages: initialMessage ? [initialMessage] : [],
	};

	await setDoc(
		doc(db, SESSIONS_COLLECTION, sessionId),
		sanitizeForFirestore(session),
	);
	return session;
}

// Get a session by ID
export async function getSession(sessionId: string): Promise<Session | null> {
	const docRef = doc(db, SESSIONS_COLLECTION, sessionId);
	const docSnap = await getDoc(docRef);

	if (!docSnap.exists()) {
		return null;
	}

	return docSnap.data() as Session;
}

// Backend session info from API
type BackendSessionInfo = {
	id: string;
	userId: string;
	title: string;
	createdAt: string;
	updatedAt: string;
	overallScore?: number;
	photoUrl?: string;
	originalPhotoUrl?: string;
	cleanEnhancedPhotoUrl?: string;
	messageCount: number;
};

// Get all sessions for a user (from backend API)
export async function getUserSessions(userId: string): Promise<Session[]> {
	try {
		const response = await fetch(
			`/api/sessions?userId=${encodeURIComponent(userId)}`,
		);

		if (!response.ok) {
			throw new Error("Failed to fetch sessions");
		}

		const data = (await response.json()) as { sessions: BackendSessionInfo[] };

		// Convert backend sessions to frontend Session type
		return (data.sessions || []).map(
			(backendSession): Session => ({
				id: backendSession.id,
				userId: backendSession.userId,
				createdAt: Timestamp.fromDate(new Date(backendSession.createdAt)),
				updatedAt: Timestamp.fromDate(new Date(backendSession.updatedAt)),
				title: backendSession.title,
				overallScore: backendSession.overallScore,
				photoUrl: toFrontendImageUrl(backendSession.photoUrl),
				originalPhotoUrl: toFrontendImageUrl(backendSession.originalPhotoUrl),
				cleanEnhancedPhotoUrl: toFrontendImageUrl(
					backendSession.cleanEnhancedPhotoUrl,
				),
				messages: [], // Messages are loaded separately when session is selected
				messageCount: backendSession.messageCount,
			}),
		);
	} catch (error) {
		console.warn(
			"Failed to fetch sessions from API, falling back to Firestore",
			error,
		);
		// Fallback to Firestore query if backend fails
		const q = query(
			collection(db, SESSIONS_COLLECTION),
			where("userId", "==", userId),
			orderBy("updatedAt", "desc"),
		);

		const querySnapshot = await getDocs(q);
		return querySnapshot.docs.map((docData) => {
			const session = docData.data() as Session;
			return {
				...session,
				photoUrl: toFrontendImageUrl(session.photoUrl),
				originalPhotoUrl: toFrontendImageUrl(session.originalPhotoUrl),
				cleanEnhancedPhotoUrl: toFrontendImageUrl(
					session.cleanEnhancedPhotoUrl,
				),
			};
		});
	}
}

// Backend session detail from API
type BackendMessageInfo = {
	role: "user" | "agent";
	content: string;
	timestamp: string;
};

type BackendSessionDetail = {
	id: string;
	userId: string;
	title: string;
	createdAt: string;
	updatedAt: string;
	overallScore?: number;
	photoUrl?: string;
	messageCount: number;
	messages: BackendMessageInfo[];
	analysisResult?: AnalysisResult;
	originalImageUrl?: string;
	cleanEnhancedImageUrl?: string;
};

// Session detail result including photo session data
export type SessionDetailResult = {
	session: Session;
	photoSession: {
		originalPreview?: string;
		enhancedPreview?: string;
		cleanEnhancedPreview?: string;
		analysis: AnalysisResult;
	} | null;
};

/**
 * Extract the original user question from an enriched message.
 * The backend enriches user messages with analysis context in the format:
 * `[この写真セッションの分析コンテキスト]\n...\n\n[ユーザーの質問]\n{original question}`
 * This function extracts just the original question part.
 */
function extractOriginalUserMessage(content: string): string {
	const userQuestionMarker = "[ユーザーの質問]";
	const markerIndex = content.indexOf(userQuestionMarker);

	if (markerIndex === -1) {
		// Not an enriched message, return as-is
		return content;
	}

	// Extract everything after the marker
	const afterMarker = content.substring(
		markerIndex + userQuestionMarker.length,
	);
	return afterMarker.trim();
}

// Get session detail including messages from backend API
export async function getSessionDetail(
	userId: string,
	sessionId: string,
): Promise<SessionDetailResult | null> {
	try {
		const response = await fetch(
			`/api/sessions/${encodeURIComponent(sessionId)}?userId=${encodeURIComponent(userId)}`,
		);

		if (!response.ok) {
			if (response.status === 404) {
				return null;
			}
			throw new Error("Failed to fetch session detail");
		}

		const data = (await response.json()) as BackendSessionDetail;

		// Convert URLs to frontend proxy paths
		const resolvedPhotoUrl = toFrontendImageUrl(data.photoUrl);
		const resolvedOriginalUrl = toFrontendImageUrl(data.originalImageUrl);
		const resolvedCleanEnhancedUrl = toFrontendImageUrl(
			data.cleanEnhancedImageUrl,
		);

		// Convert backend messages to ChatMessage[]
		// Find the first agent message to attach photo and analysis cards
		let hasAttachedCards = false;

		const messages = data.messages.map((msg, index) => {
			// Extract original user message from enriched content (if applicable)
			const displayContent =
				msg.role === "user"
					? extractOriginalUserMessage(msg.content)
					: msg.content;
			const chatMessage: ChatMessage = {
				id: `${data.id}-msg-${index}`,
				role: msg.role,
				content: displayContent,
				timestamp: Timestamp.fromDate(new Date(msg.timestamp)),
			};

			// Attach photo and analysis cards to the first agent message after user upload
			if (!hasAttachedCards && msg.role === "agent" && data.analysisResult) {
				if (!resolvedPhotoUrl) {
					console.error(
						`Enhanced image URL resolution failed for session ${data.id}: photoUrl=${data.photoUrl}`,
					);
				}
				if (!resolvedOriginalUrl) {
					console.error(
						`Original image URL resolution failed for session ${data.id}: originalImageUrl=${data.originalImageUrl}`,
					);
				}
				chatMessage.photoCard = {
					original: resolvedOriginalUrl,
					enhanced: resolvedPhotoUrl,
					cleanEnhanced: resolvedCleanEnhancedUrl,
				};
				chatMessage.analysisCard = data.analysisResult;
				hasAttachedCards = true;
			}

			return chatMessage;
		});

		// Build session object
		const session: Session = {
			id: data.id,
			userId: data.userId,
			createdAt: Timestamp.fromDate(new Date(data.createdAt)),
			updatedAt: Timestamp.fromDate(new Date(data.updatedAt)),
			title: data.title,
			overallScore: data.overallScore,
			photoUrl: resolvedPhotoUrl,
			messages,
			messageCount: data.messageCount,
		};

		// Build photo session if analysis data exists
		let photoSession: SessionDetailResult["photoSession"] = null;
		if (data.analysisResult) {
			if (!resolvedPhotoUrl) {
				console.error(
					`Cannot display enhanced image for session ${data.id}: photoUrl=${data.photoUrl} failed to resolve`,
				);
			}
			if (!resolvedOriginalUrl) {
				console.error(
					`Cannot display original image for session ${data.id}: originalImageUrl=${data.originalImageUrl} failed to resolve`,
				);
			}
			photoSession = {
				originalPreview: resolvedOriginalUrl,
				enhancedPreview: resolvedPhotoUrl,
				cleanEnhancedPreview: resolvedCleanEnhancedUrl,
				analysis: data.analysisResult,
			};
		}

		return { session, photoSession };
	} catch (error) {
		console.warn(
			"Failed to fetch session detail from API, falling back to Firestore",
			error,
		);
		// Fallback to Firestore if backend fails
		const firestoreSession = await getSession(sessionId);
		if (!firestoreSession) {
			return null;
		}

		// Try to reconstruct photo session from messages
		let photoSession: SessionDetailResult["photoSession"] = null;
		const lastAnalysisMsg = [...firestoreSession.messages]
			.reverse()
			.find((m) => m.analysisCard);
		const lastPhotoMsg = [...firestoreSession.messages]
			.reverse()
			.find((m) => m.photoCard);

		if (lastAnalysisMsg?.analysisCard && lastPhotoMsg?.photoCard) {
			photoSession = {
				originalPreview: toFrontendImageUrl(lastPhotoMsg.photoCard.original),
				enhancedPreview: toFrontendImageUrl(lastPhotoMsg.photoCard.enhanced),
				cleanEnhancedPreview: toFrontendImageUrl(
					lastPhotoMsg.photoCard.cleanEnhanced,
				),
				analysis: lastAnalysisMsg.analysisCard,
			};
		}

		// Convert session-level URLs
		const resolvedPhotoUrl = toFrontendImageUrl(firestoreSession.photoUrl);
		const resolvedOriginalPhotoUrl = toFrontendImageUrl(
			firestoreSession.originalPhotoUrl,
		);

		return {
			session: {
				...firestoreSession,
				photoUrl: resolvedPhotoUrl,
				originalPhotoUrl: resolvedOriginalPhotoUrl,
			},
			photoSession,
		};
	}
}

// Add a message to a session
export async function addMessageToSession(
	sessionId: string,
	message: ChatMessage,
): Promise<void> {
	const session = await getSession(sessionId);
	if (!session) {
		throw new Error("Session not found");
	}

	const updatedMessages = [...session.messages, message];
	await updateDoc(
		doc(db, SESSIONS_COLLECTION, sessionId),
		sanitizeForFirestore({
			messages: updatedMessages,
			updatedAt: Timestamp.now(),
		}),
	);
}

// Update session metadata (title, score, photoUrl)
export async function updateSessionMetadata(
	sessionId: string,
	metadata: {
		title?: string;
		overallScore?: number;
		photoUrl?: string;
	},
): Promise<void> {
	await updateDoc(doc(db, SESSIONS_COLLECTION, sessionId), {
		...metadata,
		updatedAt: Timestamp.now(),
	});
}

// Format session title from date
function formatSessionTitle(date: Date): string {
	return date.toLocaleDateString("ja-JP", {
		month: "long",
		day: "numeric",
		hour: "2-digit",
		minute: "2-digit",
	});
}

// Convert Firestore data to Session
export function convertToSession(data: DocumentData): Session {
	return {
		id: data.id,
		userId: data.userId,
		createdAt: data.createdAt,
		updatedAt: data.updatedAt,
		title: data.title,
		overallScore: data.overallScore,
		photoUrl: data.photoUrl,
		originalPhotoUrl: data.originalPhotoUrl,
		messages: data.messages || [],
	};
}
