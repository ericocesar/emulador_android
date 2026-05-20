/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        forest: {
          50:  '#f1f7f3',
          100: '#daeee0',
          200: '#b6dcc1',
          300: '#84c195',
          400: '#56a36c',
          500: '#36854c',
          600: '#266a3b',
          700: '#1d5630',
          800: '#184628',
          900: '#0F3329',
          950: '#0a2620',
        },
        lime: {
          accent: '#B9E938',
          'accent-hover': '#A5D928',
          'accent-dark': '#8FBE1F',
        },
        page: '#F5F4EE',
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', '-apple-system', 'sans-serif'],
      },
      boxShadow: {
        soft: '0 1px 2px rgba(15,51,41,0.04), 0 4px 12px rgba(15,51,41,0.04)',
        'soft-lg': '0 4px 24px rgba(15,51,41,0.06), 0 1px 3px rgba(15,51,41,0.04)',
      },
    },
  },
  plugins: [],
};
