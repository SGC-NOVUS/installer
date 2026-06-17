module.exports = {
  darkMode: 'class',
  content: ['web/dist/index.html'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Space Grotesk', 'ui-sans-serif', 'system-ui'],
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular']
      },
      boxShadow: {
        panel: '0 22px 80px rgba(15, 23, 42, 0.14)',
        terminal: '0 24px 90px rgba(15, 23, 42, 0.28)',
        wizard: '0 30px 120px rgba(14, 116, 144, 0.18)',
        success: '0 28px 110px rgba(22, 163, 74, 0.20)'
      },
      animation: {
        'fade-up': 'fadeUp 420ms ease-out',
        'pulse-soft': 'pulseSoft 2.8s ease-in-out infinite'
      },
      keyframes: {
        fadeUp: {
          '0%': { opacity: '0', transform: 'translateY(18px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        pulseSoft: {
          '0%, 100%': { opacity: '0.55' },
          '50%': { opacity: '1' }
        }
      }
    }
  }
};