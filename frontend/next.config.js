/** @type {import('next').NextConfig} */
// =============================================================================
// WARNING: This project uses Firebase App Hosting (Cloud Run) for SSR.
// DO NOT change output to 'export' or 'standalone'.
// DO NOT run 'firebase deploy --only hosting'.
// App Hosting automatically deploys from apphosting.yaml.
// =============================================================================
const nextConfig = {
	reactStrictMode: true,
	// output is intentionally NOT set (defaults to SSR mode for App Hosting)
	// NEVER set output: 'export' - this would break the deployment
};

module.exports = nextConfig;
