import {
	createContext,
	useContext,
	useEffect,
	useState,
	useSyncExternalStore,
} from "react";

type Theme = "dark" | "light" | "system";

type ThemeProviderProps = {
	children: React.ReactNode;
	defaultTheme?: Theme;
	storageKey?: string;
};

type ThemeProviderState = {
	theme: Theme;
	setTheme: (theme: Theme) => void;
};

const initialState: ThemeProviderState = {
	theme: "system",
	setTheme: () => null,
};

const ThemeProviderContext = createContext<ThemeProviderState>(initialState);

function applyTheme(theme: Theme) {
	const root = window.document.documentElement;
	root.classList.remove("light", "dark");

	if (theme === "system") {
		const systemTheme = window.matchMedia("(prefers-color-scheme: dark)")
			.matches
			? "dark"
			: "light";
		root.classList.add(systemTheme);
		return;
	}

	root.classList.add(theme);
}

export function ThemeProvider({
	children,
	defaultTheme = "system",
	storageKey = "elara-theme",
}: ThemeProviderProps) {
	const [theme, setTheme] = useState<Theme>(
		() => (localStorage.getItem(storageKey) as Theme) || defaultTheme,
	);

	useEffect(() => {
		applyTheme(theme);
	}, [theme]);

	// Listen for system theme changes when in "system" mode.
	useEffect(() => {
		if (theme !== "system") return;

		const mq = window.matchMedia("(prefers-color-scheme: dark)");
		const handler = () => applyTheme("system");
		mq.addEventListener("change", handler);
		return () => mq.removeEventListener("change", handler);
	}, [theme]);

	const value = {
		theme,
		setTheme: (t: Theme) => {
			localStorage.setItem(storageKey, t);
			setTheme(t);
		},
	};

	return <ThemeProviderContext value={value}>{children}</ThemeProviderContext>;
}

export const useTheme = () => {
	const context = useContext(ThemeProviderContext);
	if (context === undefined)
		throw new Error("useTheme must be used within a ThemeProvider");
	return context;
};

function subscribeSystemTheme(callback: () => void) {
	const mq = window.matchMedia("(prefers-color-scheme: dark)");
	mq.addEventListener("change", callback);
	return () => mq.removeEventListener("change", callback);
}

function getSystemThemeSnapshot() {
	return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

export function useResolvedTheme(): "dark" | "light" {
	const { theme } = useTheme();
	const systemDark = useSyncExternalStore(
		subscribeSystemTheme,
		getSystemThemeSnapshot,
	);

	if (theme === "system") return systemDark ? "dark" : "light";
	return theme;
}
