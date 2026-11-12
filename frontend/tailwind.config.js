/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  darkMode: ["class", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        canvas: "var(--ds-canvas)",
        surface: "var(--ds-surface)",
        "surface-raised": "var(--ds-surface-raised)",
        "surface-overlay": "var(--ds-surface-overlay)",
        ink: "var(--ds-ink)",
        "ink-muted": "var(--ds-ink-muted)",
        "ink-subtle": "var(--ds-ink-subtle)",
        edge: "var(--ds-edge)",
        "edge-strong": "var(--ds-edge-strong)",
        accent: "var(--ds-accent)",
        "accent-hover": "var(--ds-accent-hover)",
        "accent-subtle": "var(--ds-accent-subtle)",
        positive: "var(--ds-positive)",
        "positive-subtle": "var(--ds-positive-subtle)",
        negative: "var(--ds-negative)",
        "negative-subtle": "var(--ds-negative-subtle)",
        warn: "var(--ds-warn)",
        "warn-subtle": "var(--ds-warn-subtle)",
      },
      borderRadius: {
        ds: "var(--ds-radius)",
        "ds-sm": "var(--ds-radius-sm)",
        "ds-lg": "var(--ds-radius-lg)",
      },
      spacing: {
        "ds-sm": "var(--ds-gap-sm)",
        ds: "var(--ds-gap)",
        "ds-lg": "var(--ds-gap-lg)",
      },
    },
  },
  plugins: [],
};
