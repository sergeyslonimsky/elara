import "@testing-library/jest-dom/vitest";

Object.defineProperty(window, "localStorage", {
	value: {
		getItem: () => null,
		setItem: () => {},
		removeItem: () => {},
		clear: () => {},
	},
});

// Polyfill Element.prototype.getAnimations for happy-dom
if (typeof Element.prototype.getAnimations !== "function") {
	Element.prototype.getAnimations = () => [];
}
