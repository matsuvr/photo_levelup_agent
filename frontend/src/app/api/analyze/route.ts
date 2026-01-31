import { NextResponse } from "next/server"
import { secureLog } from "@/lib/secure-log"

export const dynamic = 'force-dynamic'

export async function POST(request: Request) {
  const backendBaseUrl = process.env.BACKEND_BASE_URL

  if (!backendBaseUrl) {
    secureLog.error("BACKEND_BASE_URL is not configured")
    return NextResponse.json(
      { error: "System configuration error: BACKEND_BASE_URL missing" },
      { status: 500 }
    )
  }

  try {
    const contentType = request.headers.get("content-type") || ""
    secureLog.info("Analyze: Received request with content-type:", contentType)

    if (!contentType.includes("multipart/form-data")) {
      secureLog.error("Analyze: Invalid content-type:", contentType)
      return NextResponse.json(
        { error: "Invalid content-type. Expected multipart/form-data" },
        { status: 400 }
      )
    }

    const formData = await request.formData()
    secureLog.info("Analyze: Forwarding request to backend")

    const backendResponse = await fetch(`${backendBaseUrl}/photo/analyze`, {
      method: "POST",
      body: formData,
    })

    secureLog.info("Analyze: Backend response status", backendResponse.status)

    const responseContentType = backendResponse.headers.get("content-type")
    const payload = responseContentType?.includes("application/json")
      ? await backendResponse.json()
      : await backendResponse.text()

    if (!backendResponse.ok) {
      secureLog.error("Analyze: Backend error", payload)
      return NextResponse.json(
        { error: typeof payload === "string" ? payload : payload?.error || "Backend error" },
        { status: backendResponse.status }
      )
    }

    // Backend now returns { jobId, status } for async processing
    return NextResponse.json(payload, { status: backendResponse.status })
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error)
    secureLog.error("Analyze: Internal error", error)
    return NextResponse.json(
      { error: `Internal Server Error: ${message}` },
      { status: 500 }
    )
  }
}
