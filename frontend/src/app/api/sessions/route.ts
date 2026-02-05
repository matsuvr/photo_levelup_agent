import { NextResponse } from "next/server";
import { secureLog } from "@/lib/secure-log";

export const dynamic = "force-dynamic";

export async function GET(request: Request) {
	const backendBaseUrl = process.env.BACKEND_BASE_URL;

	if (!backendBaseUrl) {
		secureLog.error("BACKEND_BASE_URL is not configured");
		return NextResponse.json(
			{ error: "System configuration error: BACKEND_BASE_URL missing" },
			{ status: 500 },
		);
	}

	try {
		const { searchParams } = new URL(request.url);
		const userId = searchParams.get("userId");

		if (!userId) {
			return NextResponse.json(
				{ error: "userId is required" },
				{ status: 400 },
			);
		}

		secureLog.info("Sessions: Fetching sessions for user");

		const backendResponse = await fetch(
			`${backendBaseUrl}/photo/sessions?userId=${encodeURIComponent(userId)}`,
			{
				method: "GET",
				headers: {
					"Content-Type": "application/json",
				},
			},
		);

		secureLog.info("Sessions: Backend response status", backendResponse.status);

		const responseContentType = backendResponse.headers.get("content-type");
		const payload = responseContentType?.includes("application/json")
			? await backendResponse.json()
			: await backendResponse.text();

		if (!backendResponse.ok) {
			secureLog.error("Sessions: Backend error", payload);
			return NextResponse.json(
				{
					error:
						typeof payload === "string"
							? payload
							: payload?.error || "Backend error",
				},
				{ status: backendResponse.status },
			);
		}

		return NextResponse.json(payload);
	} catch (error: unknown) {
		const message = error instanceof Error ? error.message : String(error);
		secureLog.error("Sessions: Internal error", error);
		return NextResponse.json(
			{ error: `Internal Server Error: ${message}` },
			{ status: 500 },
		);
	}
}
