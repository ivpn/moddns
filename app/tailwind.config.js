const plugin = require('tailwindcss/plugin');

module.exports = {
  content: [
    './src/**/*.{html,js,ts,jsx,tsx}',
    'app/**/*.{ts,tsx}',
    'components/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      screens: {
        xs: '360px', // iPhone narrow breakpoint helper
        // Capability-aware desktop breakpoint: only triggers on real desktop / laptop
        // (not just large-width touch tablets) by requiring hover + fine pointer.
        desktop: { raw: '(min-width:1024px) and (hover:hover) and (pointer:fine)' },
        // Optional very wide desktop refinement (future use for ultra-wide adjustments)
        xlDesktop: { raw: '(min-width:1440px) and (hover:hover) and (pointer:fine)' },
      },
      colors: {
        // Existing shadcn-ui colors
        "shadcn-ui-app-background": "var(--shadcn-ui-app-background)",
        "shadcn-ui-app-border": "var(--shadcn-ui-app-border)",
        "shadcn-ui-app-muted": "var(--shadcn-ui-app-muted)",
        "shadcn-ui-app-muted-foreground": "var(--shadcn-ui-app-muted-foreground)",
        "shadcn-ui-app-popover": "var(--shadcn-ui-app-popover)",
        "shadcn-ui-app-primary": "var(--shadcn-ui-app-primary)",
        "shadcn-ui-app-secondary": "var(--shadcn-ui-app-secondary)",
        "shadcn-ui-app-secondary-foreground": "var(--shadcn-ui-app-secondary-foreground)",
        // Existing tailwind colors
        "tailwind-colors-base-white": "var(--tailwind-colors-base-white)",
        "tailwind-colors-rdns-500": "var(--tailwind-colors-rdns-500)",
        "tailwind-colors-rdns-600": "var(--tailwind-colors-rdns-600)",
        "tailwind-colors-rdns-700": "var(--tailwind-colors-rdns-700)",
        "tailwind-colors-rdns-800": "var(--tailwind-colors-rdns-800)",
        "tailwind-colors-rdns-900": "var(--tailwind-colors-rdns-900)",
        "tailwind-colors-rdns-alpha-900": "var(--tailwind-colors-rdns-alpha-900)",
        "tailwind-colors-rdns-alpha-950": "var(--tailwind-colors-rdns-alpha-950)",
        "tailwind-colors-red-400": "var(--tailwind-colors-red-400)",
        "tailwind-colors-red-600": "var(--tailwind-colors-red-600)",
        "tailwind-colors-red-950": "var(--tailwind-colors-red-950)",
        "tailwind-colors-sky-950": "var(--tailwind-colors-sky-950)",
        "tailwind-colors-slate-100": "var(--tailwind-colors-slate-100)",
        "tailwind-colors-slate-200": "var(--tailwind-colors-slate-200)",
        "tailwind-colors-slate-300": "var(--tailwind-colors-slate-300)",
        "tailwind-colors-slate-400": "var(--tailwind-colors-slate-400)",
        "tailwind-colors-slate-50": "var(--tailwind-colors-slate-50)",
        "tailwind-colors-slate-500": "var(--tailwind-colors-slate-500)",
        "tailwind-colors-slate-600": "var(--tailwind-colors-slate-600)",
        "tailwind-colors-slate-700": "var(--tailwind-colors-slate-700)",
        "tailwind-colors-slate-800": "var(--tailwind-colors-slate-800)",
        "tailwind-colors-slate-900": "var(--tailwind-colors-slate-900)",
        "tailwind-colors-slate-950": "var(--tailwind-colors-slate-950)",
        // Variable collection colors
        "variable-collection-surface": "var(--variable-collection-surface)",
        "variable-collection-surface-foreground": "var(--variable-collection-surface-foreground)",
        "variable-collection-card": "var(--variable-collection-card)",
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        popover: {
          DEFAULT: "hsl(var(--popover))",
          foreground: "hsl(var(--popover-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
      },
      fontFamily: {
        // Existing font families
        "text-sm-leading-6-medium": "var(--text-sm-leading-6-medium-font-family)",
        "text-xs-leading-5-medium": "var(--text-xs-leading-5-medium-font-family)",
        "text-base-leading-6-normal": "var(--text-base-leading-6-normal-font-family)",
        "text-sm-leading-5-medium": "var(--text-sm-leading-5-medium-font-family)",
        "text-sm-leading-6-normal": "var(--text-sm-leading-6-normal-font-family)",
        "text-sm-leading-5-normal": "var(--text-sm-leading-5-normal-font-family)",
        "text-xs-leading-4-semibold": "var(--text-xs-leading-4-semibold-font-family)",
        "text-xs-leading-5-normal": "var(--text-xs-leading-5-normal-font-family)",
        sans: [
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
          '"Apple Color Emoji"',
          '"Segoe UI Emoji"',
          '"Segoe UI Symbol"',
          '"Noto Color Emoji"',
        ],
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
      keyframes: {
        "accordion-down": {
          from: { height: "0" },
          to: { height: "var(--radix-accordion-content-height)" },
        },
        "accordion-up": {
          from: { height: "var(--radix-accordion-content-height)" },
          to: { height: "0" },
        },
      },
      animation: {
        'accordion-down': 'accordion-down 0.2s ease-out',
        'accordion-up': 'accordion-up 0.2s ease-out',
      },
    },
    container: { center: true, padding: '1rem', screens: { sm: '640px', md: '768px', lg: '1024px', xl: '1280px', '2xl': '1400px' } },
  },
  plugins: [
    plugin(function({ addComponents, addUtilities, addVariant }) {
      addComponents({
        '.auth-shell': {
          '@apply w-full max-w-auth mx-auto': {},
        },
        '.dialog-shell': {
          '@apply w-full max-w-dialog mx-auto': {},
        },
      });
      addUtilities({
        '.safe-px': {
          paddingLeft: 'max(1rem, env(safe-area-inset-left))',
          paddingRight: 'max(1rem, env(safe-area-inset-right))',
        },
        '.safe-pt': {
          paddingTop: 'max(1rem, env(safe-area-inset-top))',
        },
        '.safe-pb': {
          paddingBottom: 'max(1rem, env(safe-area-inset-bottom))',
        },
        '.btn-icon': {
          minWidth: '44px',
          minHeight: '44px',
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
        },
      });
      // Pointer capability variants for future touch / desktop differentiation in utility classes.
      addVariant('coarse', '@media (pointer: coarse)');
      addVariant('fine', '@media (pointer: fine)');
    })
  ],
  darkMode: ["class"],
};