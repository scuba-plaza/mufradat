// The colours are declared once, as CSS variables in globals.css (light and dark), so no class names a dark variant.
const token = (name) => `rgb(var(--${name}) / <alpha-value>)`;

const steps = [0.5, 1, 1.5, 2, 2.5, 3, 3.5, 4, 5, 6, 7, 8, 9, 10, 11, 12, 14, 16, 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 64, 72, 80, 96];
const spacing = Object.fromEntries(steps.map((step) => [String(step), `calc(${step / 4}rem * var(--space))`]));

module.exports = {
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    spacing: { px: "1px", 0: "0px", ...spacing },
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
      maxWidth: {
        column: "var(--column)",
        sm: "calc(24rem * var(--space))",
        lg: "calc(32rem * var(--space))",
      },
      minHeight: { control: "var(--control-h)", "control-sm": "var(--control-h-sm)" },
      minWidth: { "control-sm": "var(--control-h-sm)" },
      // Arabic with its vowel marks needs more line height than Latin text of the same size, so every
      // Arabic size carries its own: rows and fields, a hero word, a sentence, and the headword of a card.
      fontSize: {
        xs: ["var(--text-xs)", "1.3333"],
        sm: ["var(--text-sm)", "1.4286"],
        base: ["var(--text-base)", "1.5"],
        lg: ["var(--text-lg)", "1.5556"],
        xl: ["var(--text-xl)", "1.4"],
        "2xl": ["var(--text-2xl)", "1.3333"],
        "ar-sm": ["var(--ar-sm)", "1.4"],
        "ar-stacked": ["var(--ar-sm)", "1.9"],
        "ar-md": ["var(--ar-md)", "1.7"],
        "ar-lg": ["var(--ar-lg)", "1.8"],
        "ar-xl": ["var(--ar-xl)", "1.5"],
      },
    },
  },
  plugins: [],
};
