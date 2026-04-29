import path from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
	plugins: [react()],
	test: {
		globals: true,
		environment: "happy-dom",
		setupFiles: ["./src/test/setup.ts"],
		css: true,
		reporters: ["default", "junit"],
		outputFile: { junit: "./test-results/junit.xml" },
		coverage: {
			provider: "v8",
			reporter: ["text", "json", "html", "lcov"],
			include: ["src/**/*.{ts,tsx}"],
			exclude: [
				"src/**/*.d.ts",
				"src/test/**",
				"src/**/*.test.{ts,tsx}",
				"src/gen/**",
				"src/components/ui/**",
			],
		},
	},
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
	},
});
