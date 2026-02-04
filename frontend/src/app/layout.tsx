import "./globals.css";
import type { Metadata } from "next";
import { AuthProvider } from "@/context/AuthContext";

export const metadata: Metadata = {
	title: "Photo Coach | AI写真コーチング",
	description:
		"あなたの写真をAIが分析し、プロ級の仕上がりへ導く改善アドバイスを提案します。",
	icons: {
		icon: "/icon.png",
		apple: "/icon.png",
	},
};

export default function RootLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<html lang="ja">
			<body>
				<AuthProvider>{children}</AuthProvider>
			</body>
		</html>
	);
}
