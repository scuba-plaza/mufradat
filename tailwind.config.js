// The colours are declared once, as CSS variables in globals.css (light and dark), so no class names a dark variant.
const token = (name) => `rgb(var(--${name}) / <alpha-value>)`;

module.exports = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: Object.fromEntries(
        [
          "paper", "surface", "hover", "ink", "quiet", "rule", "rule-strong", "accent", "focus", "primary", "on-primary",
          "highlight", "warn", "danger", "progress", "ok-bg", "ok-ink", "wip-bg", "wip-ink",
        ].map((name) => [name, token(name)]),
      ),
      fontFamily: {
        sans: [
          "ui-sans-serif", "-apple-system", "BlinkMacSystemFont", '"Segoe UI"', "Helvetica", "Arial", "sans-serif",
          '"Apple Color Emoji"', '"Segoe UI Emoji"', "var(--font-arabic)",
        ],
      },
      letterSpacing: { title: "-0.01em" },
      borderRadius: {
        control: "var(--radius-control)",
        surface: "var(--radius-surface)",
        modal: "var(--radius-modal)",
      },
      boxShadow: { modal: "var(--shadow-modal)" },
      minHeight: { control: "var(--control-h)", "control-sm": "var(--control-h-sm)" },
      minWidth: { "control-sm": "var(--control-h-sm)" },
      // Arabic with its vowel marks needs more line height than Latin text of the same size, so every
      // Arabic size carries its own: rows and fields, a hero word, a sentence, and the headword of a card.
      fontSize: {
        "ar-sm": ["1.5rem", "1.4"],
        "ar-md": ["2.25rem", "1.7"],
        "ar-lg": ["clamp(1.75rem, 7vw, 2.5rem)", "1.8"],
        "ar-xl": ["clamp(3rem, 14vw, 4.5rem)", "1.7"],
      },
    },
  },
  plugins: [],
};
