/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        jade: {
          50: "var(--jade-50)",
          100: "var(--jade-100)",
          200: "var(--jade-200)",
          300: "var(--jade-300)",
          400: "var(--jade-400)",
          500: "var(--jade-500)",
          600: "var(--jade-600)",
          700: "var(--jade-700)",
          800: "var(--jade-800)",
          900: "var(--jade-900)",
        },
        gray: {
          0: "var(--gray-0)",
          50: "var(--gray-50)",
          100: "var(--gray-100)",
          200: "var(--gray-200)",
          300: "var(--gray-300)",
          400: "var(--gray-400)",
          500: "var(--gray-500)",
          600: "var(--gray-600)",
          700: "var(--gray-700)",
          800: "var(--gray-800)",
          900: "var(--gray-900)",
        },
        surface: {
          app: "var(--surface-app)",
          card: "var(--surface-card)",
          hover: "var(--surface-hover)",
          active: "var(--surface-active)",
        },
        success: {
          DEFAULT: "var(--color-success)",
          fg: "var(--color-success-fg)",
          bg: "var(--color-success-bg)",
          border: "var(--color-success-border)",
        },
        warning: {
          DEFAULT: "var(--color-warning)",
          fg: "var(--color-warning-fg)",
          bg: "var(--color-warning-bg)",
          border: "var(--color-warning-border)",
        },
        danger: {
          DEFAULT: "var(--color-danger)",
          fg: "var(--color-danger-fg)",
          bg: "var(--color-danger-bg)",
          border: "var(--color-danger-border)",
        },
        info: {
          DEFAULT: "var(--color-info)",
          fg: "var(--color-info-fg)",
          bg: "var(--color-info-bg)",
          border: "var(--color-info-border)",
        },
        neutral: {
          DEFAULT: "var(--color-neutral)",
          fg: "var(--color-neutral-fg)",
          bg: "var(--color-neutral-bg)",
          border: "var(--color-neutral-border)",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
      },
      fontSize: {
        xs: ["11px", { lineHeight: "1.5" }],
        sm: ["12px", { lineHeight: "1.5" }],
        base: ["13px", { lineHeight: "1.5" }],
        md: ["14px", { lineHeight: "1.6" }],
        lg: ["16px", { lineHeight: "1.4" }],
        xl: ["20px", { lineHeight: "1.3" }],
        "2xl": ["24px", { lineHeight: "1.2" }],
        "3xl": ["30px", { lineHeight: "1.2" }],
      },
      borderRadius: {
        sm: "4px",
        md: "6px",
        lg: "8px",
        xl: "12px",
        full: "9999px",
      },
      boxShadow: {
        xs: "0 1px 2px rgba(16,24,40,0.05)",
        sm: "0 1px 3px rgba(16,24,40,0.10), 0 1px 2px rgba(16,24,40,0.06)",
        md: "0 4px 8px rgba(16,24,40,0.08), 0 2px 4px rgba(16,24,40,0.04)",
        lg: "0 12px 24px rgba(16,24,40,0.10), 0 4px 8px rgba(16,24,40,0.06)",
        xl: "0 24px 48px rgba(16,24,40,0.12)",
      },
      spacing: {
        13: "52px",
        50: "200px",
      },
      width: {
        sidebar: "200px",
        drawer: "480px",
      },
      height: {
        13: "52px",
      },
      animation: {
        fadeIn: "fadeIn 150ms ease",
        slideInRight: "slideInRight 180ms ease",
        slideUp: "slideUp 150ms ease",
      },
      keyframes: {
        fadeIn: {
          from: { opacity: "0" },
          to: { opacity: "1" },
        },
        slideInRight: {
          from: { transform: "translateX(100%)" },
          to: { transform: "translateX(0)" },
        },
        slideUp: {
          from: { transform: "translateY(8px)", opacity: "0" },
          to: { transform: "translateY(0)", opacity: "1" },
        },
      },
    },
  },
  plugins: [],
};
