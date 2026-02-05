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
import { getDownloadURL, getStorage, ref } from "firebase/storage";
import { db } from "./firebase";

export type ChatMessage = {
	id: string;
	role: "user" | "agent";
	content: string;
	timestamp: Timestamp;
	analysisCard?: AnalysisResult;
	photoCard?: { original: string; enhanced: string };
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
	messages: ChatMessage[];
	messageCount?: number;
};

const SESSIONS_COLLECTION = "sessions";

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

// Helper function to wrap a promise with a timeout
function withTimeout<T>(
	promise: Promise<T>,
	timeoutMs: number,
	errorMessage = "Operation timed out",
): Promise<T> {
	return Promise.race([
		promise,
		new Promise<T>((_, reject) =>
			setTimeout(() => reject(new Error(errorMessage)), timeoutMs),
		),
	]);
}

// Helper to resolve gs:// URLs to HTTPS download URLs
// Returns undefined if resolution fails (CORS errors, timeout, etc.)
async function resolveStorageUrl(
	url: string | undefined,
	timeoutMs = 5000, // 5 second timeout to prevent long waits
): Promise<string | undefined> {
	if (!url) return undefined;

	// If it's already an HTTPS URL, return it directly
	if (url.startsWith("https://") || url.startsWith("http://")) {
		return url;
	}

	// If it's a gs:// URL, try to resolve it
	if (url.startsWith("gs://")) {
		try {
			const storage = getStorage();
			const storageRef = ref(storage, url);
			// Add timeout to prevent long waits due to CORS issues or network problems
			return await withTimeout(
				getDownloadURL(storageRef),
				timeoutMs,
				`Timeout resolving storage URL: ${url}`,
			);
		} catch (error) {
			// Log the error but return undefined so UI can show a fallback
			console.warn(`Failed to resolve storage URL: ${url}`, error);
			// Return undefined instead of original gs:// URL
			// gs:// URLs cannot be loaded by browsers directly
			return undefined;
		}
	}

	// For other URL schemes (data:, blob:, etc.), return as-is
	return url;
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
		// Use Promise.allSettled to ensure all sessions are processed even if some URL resolutions fail
		const sessionResults = await Promise.allSettled(
			(data.sessions || []).map(async (backendSession): Promise<Session> => {
				// Resolve URLs with individual error handling - don't let one failure block others
				const [photoUrl, originalPhotoUrl] = await Promise.all([
					resolveStorageUrl(backendSession.photoUrl).catch(() => undefined),
					resolveStorageUrl(backendSession.originalPhotoUrl).catch(
						() => undefined,
					),
				]);

				return {
					id: backendSession.id,
					userId: backendSession.userId,
					createdAt: Timestamp.fromDate(new Date(backendSession.createdAt)),
					updatedAt: Timestamp.fromDate(new Date(backendSession.updatedAt)),
					title: backendSession.title,
					overallScore: backendSession.overallScore,
					photoUrl,
					originalPhotoUrl,
					messages: [], // Messages are loaded separately when session is selected
					messageCount: backendSession.messageCount,
				};
			}),
		);

		// Extract successful results, filter out failures
		return sessionResults
			.filter(
				(result): result is PromiseFulfilledResult<Session> =>
					result.status === "fulfilled",
			)
			.map((result) => result.value);
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
		const rawSessions = querySnapshot.docs.map(
			(docData) => docData.data() as Session,
		);

		// Resolve URLs for Firestore sessions too (they might have gs:// URLs)
		const resolvedSessions = await Promise.allSettled(
			rawSessions.map(async (session): Promise<Session> => {
				const [photoUrl, originalPhotoUrl] = await Promise.all([
					resolveStorageUrl(session.photoUrl).catch(() => undefined),
					resolveStorageUrl(session.originalPhotoUrl).catch(() => undefined),
				]);
				return {
					...session,
					photoUrl,
					originalPhotoUrl,
				};
			}),
		);

		return resolvedSessions
			.filter(
				(result): result is PromiseFulfilledResult<Session> =>
					result.status === "fulfilled",
			)
			.map((result) => result.value);
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
};

// Session detail result including photo session data
export type SessionDetailResult = {
	session: Session;
	photoSession: {
		originalPreview: string;
		enhancedPreview: string;
		analysis: AnalysisResult;
	} | null;
};

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

		// Resolve session level URLs with error handling
		const [resolvedPhotoUrl, resolvedOriginalUrl] = await Promise.all([
			resolveStorageUrl(data.photoUrl).catch(() => undefined),
			resolveStorageUrl(data.originalImageUrl).catch(() => undefined),
		]);

		// Convert backend messages to ChatMessage[]
		// Find the first agent message to attach photo and analysis cards
		let hasAttachedCards = false;

		// We need to process messages asynchronously to resolve URLs in cards
		const messages = await Promise.all(
			data.messages.map(async (msg, index) => {
				const chatMessage: ChatMessage = {
					id: `${data.id}-msg-${index}`,
					role: msg.role,
					content: msg.content,
					timestamp: Timestamp.fromDate(new Date(msg.timestamp)),
				};

				// Attach photo and analysis cards to the first agent message after user upload
				if (
					!hasAttachedCards &&
					msg.role === "agent" &&
					data.analysisResult &&
					resolvedPhotoUrl
				) {
					chatMessage.photoCard = {
						original: resolvedOriginalUrl || resolvedPhotoUrl,
						enhanced: resolvedPhotoUrl,
					};
					chatMessage.analysisCard = data.analysisResult;
					hasAttachedCards = true;
				}

				return chatMessage;
			}),
		);

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
		if (data.analysisResult && resolvedPhotoUrl) {
			photoSession = {
				originalPreview: resolvedOriginalUrl || resolvedPhotoUrl,
				enhancedPreview: resolvedPhotoUrl,
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
			// Resolve URLs from photoCard (they might be gs:// URLs)
			const [resolvedOriginal, resolvedEnhanced] = await Promise.all([
				resolveStorageUrl(lastPhotoMsg.photoCard.original).catch(
					() => undefined,
				),
				resolveStorageUrl(lastPhotoMsg.photoCard.enhanced).catch(
					() => undefined,
				),
			]);

			// Only create photoSession if we have valid resolved URLs
			if (resolvedOriginal && resolvedEnhanced) {
				photoSession = {
					originalPreview: resolvedOriginal,
					enhancedPreview: resolvedEnhanced,
					analysis: lastAnalysisMsg.analysisCard,
				};
			}
		}

		// Also resolve session-level URLs
		const [resolvedPhotoUrl, resolvedOriginalPhotoUrl] = await Promise.all([
			resolveStorageUrl(firestoreSession.photoUrl).catch(() => undefined),
			resolveStorageUrl(firestoreSession.originalPhotoUrl).catch(
				() => undefined,
			),
		]);

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
