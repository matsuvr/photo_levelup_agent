import { NextResponse } from "next/server"

const backendBaseUrl = process.env.BACKEND_BASE_URL

export async function POST(request: Request) {
  if (!backendBaseUrl) {
    return NextResponse.json(
      { error: "BACKEND_BASE_URL is not configured." },
      { status: 500 }
    )
  }

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
}
