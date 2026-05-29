/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      colors: {
        pd: {
          green: "#1A7F37",
          "green-dark": "#156B2E",
          bg: "#F4F5F7",
          surface: "#FFFFFF",
          border: "#E3E6EA",
          text: "#25292E",
          muted: "#6B7280",
          stage: "#EDEFF2",
          blue: "#2D7FF9",
          amber: "#E8A33D",
          red: "#D64545",
        },
      },
      fontFamily: {
        sans: ['"Source Sans 3"', "system-ui", "sans-serif"],
      },
      borderRadius: {
        DEFAULT: "4px",
      },
      fontSize: {
        sm: ["13px", "18px"],
        base: ["14px", "20px"],
      },
    },
  },
  plugins: [],
};
