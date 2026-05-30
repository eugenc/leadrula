/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        jade: {
          50: "#F0FAF5",
          100: "#D6F2E4",
          200: "#AEDEC9",
          300: "#77C5A8",
          400: "#3DAB83",
          500: "#1A9268",
          600: "#157A57",
          700: "#0F5E41",
          800: "#0A3F2C",
          900: "#052015",
        },
        gray: {
          0: "#FFFFFF",
          50: "#F4F5F7",
          100: "#EAECF0",
          200: "#D0D5DD",
          300: "#98A2B3",
          400: "#667085",
          500: "#475467",
          600: "#344054",
          700: "#1D2939",
          800: "#101828",
          900: "#0D1117",
        },
        success: {
          DEFAULT: "var(--color-success)",
          bg: "var(--color-success-bg)",
          border: "var(--color-success-border)",
        },
        warning: {
          DEFAULT: "var(--color-warning)",
          bg: "var(--color-warning-bg)",
          border: "var(--color-warning-border)",
        },
        danger: {
          DEFAULT: "var(--color-danger)",
          bg: "var(--color-danger-bg)",
          border: "var(--color-danger-border)",
        },
        info: {
          DEFAULT: "var(--color-info)",
          bg: "var(--color-info-bg)",
          border: "var(--color-info-border)",
        },
        neutral: {
          DEFAULT: "var(--color-neutral)",
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
