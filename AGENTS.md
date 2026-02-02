# Agent Guidelines & Project Rules

This document outlines the mandatory rules and best practices for the AI agent working on this project.

gemini-3-pro-image-preview, gemini-3-flash-preview はすでにリリースされているモデルです。勝手にモデル名を書き換えないように！！！！

GoバックエンドがGeminiとやりとりするときは[Google Gen AI Go SDK](https://pkg.go.dev/google.golang.org/genai#section-readme)を用いること。Generative ai sdkはすでに更新されなくなったものなので使わないこと！

## 1. Technology Stack & Tools
- **TypeScript Linting/Formatting**: Use **Biome** explicitly. **Do NOT use ESLint**.
- **TypeScript Types**: Enforce strict typing.
  - **No `any`**: The use of `any` (explicit or implicit) is strictly prohibited.
- **Go**: Use standard Go linters and formatters.

## 2. Implementation Standards
- **Google ADK**: Always refer to the [Google ADK Documentation](https://google.github.io/adk-docs/get-started/go/) and strictly follow its best practices.
- **Design Alignment**: Implement features strictly based on the specifications in `design.md`.
- **Next.js Best Practices**: Adhere to the best practices modeled in the `next-best-practices` skill.

## 3. UI/UX Philosophy
- **Mobile-First**: The application is designed primarily for smartphones.
  - Prioritize mobile usability in all UI/UX decisions.
  - Desktop/PC browser support is a secondary "nice-to-have" (bonus) and should not compromise the mobile experience.

## 4. Project Context (Hackathon)
- **Scope**: This is a submission for a hackathon, not a production-ready commercial product.
- **Backward Compatibility**: **Not required**. Breaking changes are acceptable.
- **Reliability vs. Efficiency**:
  - Service downtime caused by deployment failures is clear and acceptable.
  - **Priority**: Do NOT write wasteful code or consume resources for the sake of redundancy, high availability, or continuity.
  - **Cost**: This is a hobby project; minimize resource consumption and cost. Avoid over-engineering.
