/** @type {import('next').NextConfig} */
// =============================================================================
// WARNING: This project uses Firebase App Hosting (Cloud Run).
// The Dockerfile expects 'standalone' output to build the container image.
// DO NOT change output to 'export'.
// =============================================================================
const nextConfig = {
	reactStrictMode: true,
	output: "standalone",
	// NEVER set output: 'export' - this would break the deployment
};

module.exports = nextConfig;
