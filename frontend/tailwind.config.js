/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        dracula: {
          bg: '#101315',
          current: '#171b1f',
          selection: '#22282e',
          fg: '#eef3f8',
          comment: '#8d9aa8',
          cyan: '#34aaf4',
          green: '#44df83',
          orange: '#ff9f0a',
          pink: '#44cfc1',
          purple: '#1c8ad7',
          red: '#ff7b72',
          yellow: '#eef3f8',
        },
      },
    },
  },
  plugins: [],
}
