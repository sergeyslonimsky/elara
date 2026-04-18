import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
	plugins: [react(), tailwindcss()],
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
	},
	server: {
		port: 3000,
		proxy: {
			// ConnectRPC services
			"/elara.": {
				target: "http://localhost:8080",
				changeOrigin: true,
			},
		},
	},
	build: {
		outDir: "dist",
		assetsDir: "assets",
		emptyOutDir: true,
	},
});
