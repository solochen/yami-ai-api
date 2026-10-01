import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: ["./src/**/*.{js,ts,jsx,tsx,mdx}"],
  theme: {
    extend: {
      colors: {
        primary: "#FF8711",
        secondary: "#C9B7AC",
        accent: "#F2C18D",
        danger: "#FF4D4F",
        dark: "#160B03",
        surface: "#F3EEE9",
      },
    },
  },
  plugins: [],
};
export default config;
