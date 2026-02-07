import type { NextRequest } from "next/server";
import { secureLog } from "@/lib/secure-log";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
	const backendBaseUrl = process.env.BACKEND_BASE_URL;

	if (!backendBaseUrl) {
		secureLog.error("BACKEND_BASE_URL is not configured");
		return new Response("System configuration error", { status: 500 });
	}

	const objectName = request.nextUrl.searchParams.get("object");
	if (!objectName) {
		return new Response("object parameter is required", { status: 400 });
	}

	try {
		const backendUrl = `${backendBaseUrl}/photo/image?object=${encodeURIComponent(objectName)}&download=true`;
		secureLog.info("Image proxy: forwarding to backend");

		const backendResponse = await fetch(backendUrl);

		if (!backendResponse.ok) {
			secureLog.error("Image proxy: backend error", backendResponse.status);
			return new Response("Image not found", {
				status: backendResponse.status,
			});
		}

		const headers = new Headers();
		const contentType = backendResponse.headers.get("content-type");
		if (contentType) headers.set("Content-Type", contentType);

		const contentDisposition = backendResponse.headers.get(
			"content-disposition",
		);
		if (contentDisposition)
			headers.set("Content-Disposition", contentDisposition);

		const contentLength = backendResponse.headers.get("content-length");
		if (contentLength) headers.set("Content-Length", contentLength);

		return new Response(backendResponse.body, {
			status: 200,
			headers,
		});
	} catch (error: unknown) {
		const message = error instanceof Error ? error.message : String(error);
		secureLog.error("Image proxy: internal error", error);
		return new Response(`Internal Server Error: ${message}`, { status: 500 });
	}
}
