import { Config } from 'tailwindcss';

/**
 * Core brand colors – used directly in Tailwind utilities.
 */
export const brandColors = {
  primary: '#0F62FE',
  secondary: '#6F6F6F',
  success: '#24A148',
  danger: '#DA1E28',
  background: '#F4F4F4',
  surface: '#FFFFFF',
} as const;

/**
 * Dark‑mode palette expressed as CSS custom properties.
 * These variables are injected into `:root` when the user enables dark mode
 * (handled by `applyDarkMode` below) and referenced from Tailwind via
 * `var(--color-…)` tokens.
 */
export const darkModeVariables: Record<string, string> = {
  '--color-background': '#1A1A1A',
  '--color-surface': '#2A2A2A',
  '--color-primary': brandColors.primary,
  '--color-secondary': '#A8A8A8',
  '--color-success': brandColors.success,
  '--color-danger': brandColors.danger,
  '--color-text-primary': '#E0E0E0',
  '--color-text-secondary': '#B0B0B0',
};

/**
 * Shadow scale that matches the design system’s elevation guidelines.
 */
export const elevationShadows = {
  sm: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
  DEFAULT: '0 1px 3px 0 rgba(0, 0, 0, 0.10), 0 1px 2px 0 rgba(0, 0, 0, 0.06)',
  md: '0 4px 6px -1px rgba(0, 0, 0, 0.10), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
  lg: '0 10px 15px -3px rgba(0, 0, 0, 0.10), 0 4px 6px -2px rgba(0, 0, 0, 0.05)',
  xl: '0 20px 25px -5px rgba(0, 0, 0, 0.10), 0 10px 10px -5px rgba(0, 0, 0, 0.04)',
  '2xl': '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
  inner: 'inset 0 2px 4px 0 rgba(0, 0, 0, 0.06)',
  none: 'none',
} as const;

/**
 * Tailwind configuration fragment that can be imported from `tailwind.config.js`.
 * It extends the default theme with the brand palette, dark‑mode CSS variable
 * references, and the elevation shadow scale.
 */
export const tailwindTheme: Config = {
  darkMode: 'class', // toggled via the `dark` class on <html>
  theme: {
    extend: {
      colors: {
        // Direct brand tokens
        primary: brandColors.primary,
        secondary: brandColors.secondary,
        success: brandColors.success,
        danger: brandColors.danger,
        // Semantic tokens that resolve to CSS variables (dark‑mode aware)
        background: 'var(--color-background)',
        surface: 'var(--color-surface)',
        'text-primary': 'var(--color-text-primary)',
        'text-secondary': 'var(--color-text-secondary)',
      },
      boxShadow: elevationShadows,
    },
  },
};

/**
 * Apply or remove dark‑mode CSS variables.
 *
 * This helper is used by the ThemeProvider (or any component that reacts to the
 * user’s theme preference). It safely guards against missing `document` (e.g.
 * during server‑side rendering) and logs any unexpected errors without breaking
 * the UI.
 *
 * @param enabled - `true` to enable dark mode, `false` to revert to light mode.
 */
export const applyDarkMode = (enabled: boolean): void => {
  if (typeof window === 'undefined' || !window.document) {
    // Server‑side rendering – nothing to do.
    return;
  }

  try {
    const root = window.document.documentElement;

    if (enabled) {
      Object.entries(darkModeVariables).forEach(([key, value]) => {
        root.style.setProperty(key, value);
      });
      root.classList.add('dark');
    } else {
      Object.keys(darkModeVariables).forEach((key) => {
        root.style.removeProperty(key);
      });
      root.classList.remove('dark');
    }
  } catch (err) {
    // eslint-disable-next-line no-console
    console.error('Failed to toggle dark mode:', err);
  }
};

export default tailwindTheme;