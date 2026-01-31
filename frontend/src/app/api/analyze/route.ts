import { NextResponse } from "next/server"

export const dynamic = 'force-dynamic'

export async function POST(request: Request) {
  const backendBaseUrl = process.env.BACKEND_BASE_URL

  if (!backendBaseUrl) {
    console.error("BACKEND_BASE_URL is not configured")
    return NextResponse.json(
      { error: "System configuration error: BACKEND_BASE_URL missing" },
      { status: 500 }
    )
  }

  try {
    const contentType = request.headers.get("content-type") || ""
    console.log("Analyze: Received request with content-type:", contentType)

    if (!contentType.includes("multipart/form-data")) {
      console.error("Analyze: Invalid content-type:", contentType)
      return NextResponse.json(
        { error: "Invalid content-type. Expected multipart/form-data" },
        { status: 400 }
      )
    }

    const formData = await request.formData()
    console.log("Analyze: Forwarding request to", `${backendBaseUrl}/photo/analyze`)

    const backendResponse = await fetch(`${backendBaseUrl}/photo/analyze`, {
      method: "POST",
      body: formData,
    })

    console.log("Analyze: Backend response status", backendResponse.status)

    const responseContentType = backendResponse.headers.get("content-type")
    const payload = responseContentType?.includes("application/json")
      ? await backendResponse.json()
      : await backendResponse.text()

    if (!backendResponse.ok) {
      console.error("Analyze: Backend error", payload)
      return NextResponse.json(
        { error: typeof payload === "string" ? payload : payload?.error || "Backend error" },
        { status: backendResponse.status }
      )
    }

    // Backend now returns { jobId, status } for async processing
    return NextResponse.json(payload, { status: backendResponse.status })
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error)
    console.error("Analyze: Internal error", error)
    return NextResponse.json(
      { error: `Internal Server Error: ${message}` },
      { status: 500 }
    )
  }
}
