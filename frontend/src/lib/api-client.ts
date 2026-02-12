import { auth } from "@/lib/firebase";

export async function getAuthHeaders(): Promise<HeadersInit> {
	const user = auth.currentUser;
	if (!user) {
		return {};
	}
	try {
		const idToken = await user.getIdToken();
		return {
			Authorization: `Bearer ${idToken}`,
		};
	} catch (error) {
		console.error("Failed to get ID token:", error);
		return {};
	}
}

export async function fetchWithAuth(
	url: string,
	options: RequestInit = {},
): Promise<Response> {
	const authHeaders = await getAuthHeaders();
	return fetch(url, {
		...options,
		headers: {
			...options.headers,
			...authHeaders,
		},
	});
}
