/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./views/**/*.templ",
    "./public/**/*.{html,js}"
  ],
  theme: {
    extend: {},
  },
  plugins: [require("daisyui")],
  daisyui: {
    themes: ["emerald", "dark", "light"],
    darkTheme: "dark",
  },
}
