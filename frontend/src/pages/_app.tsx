import type { AppProps } from 'next/app';
import Head from 'next/head';
import { useEffect, useState, createContext, ReactNode } from 'react';
import '../styles/globals.css';
import useAuth from '@/hooks/useAuth';

/* -------------------------------------------------------------------------- */
/* Theme Context & Provider                                                    */
/* -------------------------------------------------------------------------- */

export const ThemeContext = createContext<{
  theme: 'light' | 'dark';
  toggleTheme: () => void;
}>({
  theme: 'light',
  toggleTheme: () => {},
});

const ThemeProvider = ({ children }: { children: ReactNode }) => {
  const [theme, setTheme] = useState<'light' | 'dark'>('light');

  // Initialise theme from localStorage or system preference
  useEffect(() => {
    if (typeof window === 'undefined') return;

    const stored = localStorage.getItem('theme') as 'light' | 'dark' | null;
    if (stored) {
      setTheme(stored);
    } else {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      setTheme(prefersDark ? 'dark' : 'light');
    }
  }, []);

  // Apply Tailwind dark class and persist choice
  useEffect(() => {
    if (typeof document === 'undefined') return;

    const root = document.documentElement;
    if (theme === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }
    localStorage.setItem('theme', theme);
  }, [theme]);

  const toggleTheme = () => setTheme(prev => (prev === 'dark' ? 'light' : 'dark'));

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme }}>
      {children}
    </ThemeContext.Provider>
  );
};

/* -------------------------------------------------------------------------- */
/* Auth Context & Provider                                                     */
/* -------------------------------------------------------------------------- */

export const AuthContext = createContext<{
  user: any;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refreshToken: () => Promise<void>;
}>({
  user: null,
  loading: false,
  login: async () => {},
  logout: () => {},
  refreshToken: async () => {},
});

const AuthProvider = ({ children }: { children: ReactNode }) => {
  const { user, loading, login, logout, refreshToken } = useAuth();

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, refreshToken }}>
      {children}
    </AuthContext.Provider>
  );
};

/* -------------------------------------------------------------------------- */
/* Custom App Component                                                       */
/* -------------------------------------------------------------------------- */

function MyApp({ Component, pageProps }: AppProps) {
  return (
    <>
      <Head>
        <title>FinSight AI</title>
        <meta
          name="description"
          content="Intelligent Invoice & Receipt Automation Platform"
        />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>

      <ThemeProvider>
        <AuthProvider>
          <Component {...pageProps} />
        </AuthProvider>
      </ThemeProvider>
    </>
  );
}

export default MyApp;