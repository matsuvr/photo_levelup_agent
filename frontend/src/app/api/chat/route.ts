import { NextResponse } from "next/server"
import { secureLog } from "@/lib/secure-log"

export const dynamic = 'force-dynamic'

export async function POST(request: Request) {
  const backendBaseUrl = process.env.BACKEND_BASE_URL

  if (!backendBaseUrl) {
    return NextResponse.json(
      { error: "BACKEND_BASE_URL is not configured." },
      { status: 500 }
    )
  }

  try {
    const payload = await request.json()
    const response = await fetch(`${backendBaseUrl}/photo/chat`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    })

    const contentType = response.headers.get("content-type")
    const data = contentType?.includes("application/json")
      ? await response.json()
      : await response.text()

    if (!response.ok) {
      return NextResponse.json(
        { error: typeof data === "string" ? data : data?.error },
        { status: response.status }
      )
    }

    return NextResponse.json(data)
  } catch (error: any) {
    secureLog.error("Chat error:", error)
    return NextResponse.json(
      { error: `Internal Server Error: ${error.message}` },
      { status: 500 }
    )
  }
}
