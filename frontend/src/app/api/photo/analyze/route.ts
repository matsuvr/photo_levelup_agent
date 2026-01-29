import { NextResponse } from "next/server"

const backendBaseUrl = process.env.BACKEND_BASE_URL

export async function POST(request: Request) {
  if (!backendBaseUrl) {
    return NextResponse.json(
      { error: "BACKEND_BASE_URL is not configured." },
      { status: 500 }
    )
  }

  const formData = await request.formData()
  try {
    const response = await fetch(`${backendBaseUrl}/photo/analyze`, {
      method: "POST",
      body: formData,
    })

    const contentType = response.headers.get("content-type")
    const payload = contentType?.includes("application/json")
      ? await response.json()
      : await response.text()

    if (!response.ok) {
      return NextResponse.json(
        { error: typeof payload === "string" ? payload : payload?.error },
        { status: response.status }
      )
    }

    return NextResponse.json(payload)
  } catch (error) {
    console.error("Backend fetch failed", error)
    return NextResponse.json(
      { error: "バックエンドに接続できませんでした。" },
      { status: 502 }
    )
  }
}
