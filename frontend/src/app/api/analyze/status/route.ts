import { NextResponse } from "next/server"
import { secureLog } from "@/lib/secure-log"

export const dynamic = 'force-dynamic'

export async function GET(request: Request) {
    const backendBaseUrl = process.env.BACKEND_BASE_URL

    if (!backendBaseUrl) {
        secureLog.error("BACKEND_BASE_URL is not configured")
        return NextResponse.json(
            { error: "System configuration error: BACKEND_BASE_URL missing" },
            { status: 500 }
        )
    }

    try {
        const { searchParams } = new URL(request.url)
        const jobId = searchParams.get("jobId")

        if (!jobId) {
            return NextResponse.json(
                { error: "jobId is required" },
                { status: 400 }
            )
        }

        secureLog.info("AnalyzeStatus: Checking job", jobId)

        const backendResponse = await fetch(
            `${backendBaseUrl}/photo/analyze/status?jobId=${encodeURIComponent(jobId)}`,
            { method: "GET" }
        )

        secureLog.info("AnalyzeStatus: Backend response status", backendResponse.status)

        const payload = await backendResponse.json()

        if (!backendResponse.ok) {
            secureLog.error("AnalyzeStatus: Backend error", payload)
            return NextResponse.json(
                { error: payload?.error || "Backend error" },
                { status: backendResponse.status }
            )
        }

        return NextResponse.json(payload)
    } catch (error: unknown) {
        const message = error instanceof Error ? error.message : String(error)
        secureLog.error("AnalyzeStatus: Internal error", error)
        return NextResponse.json(
            { error: `Internal Server Error: ${message}` },
            { status: 500 }
        )
    }
}
