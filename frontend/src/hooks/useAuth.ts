import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  ReactNode,
  useCallback,
} from 'react';
import api from '../utils/api';

/**
 * Types
 */
export interface User {
  id: string;
  email: string;
  role: 'admin' | 'user';
  preferences?: {
    theme?: 'light' | 'dark';
    language?: string;
  };
}

interface AuthContextValue {
  /** Currently authenticated user or null */
  user: User | null;
  /** True while an auth request (login/refresh) is in progress */
  loading: boolean;
  /** Last error message from an auth operation */
  error: string | null;
  /** Perform login with email & password */
  login: (email: string, password: string) => Promise<void>;
  /** Perform logout */
  logout: () => Promise<void>;
  /** Refresh the session (used internally and can be called manually) */
  refresh: () => Promise<void>;
  /** Convenience flag */
  isAuthenticated: boolean;
}

/**
 * Context
 */
const AuthContext = createContext<AuthContextValue | undefined>(undefined);

/**
 * Provider
 */
export const AuthProvider = ({
  children,
}: {
  children: ReactNode;
}): JSX.Element => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  /**
   * Refresh the session – called on mount and periodically.
   */
  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // The server validates the HttpOnly JWT cookie and returns the user payload.
      const data = await api.post<{ user: User }>('/auth/refresh', {});
      setUser(data.user);
    } catch (err: any) {
      // If refresh fails we treat the user as unauthenticated.
      setUser(null);
      if (err?.message) {
        setError(err.message);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  /**
   * Login flow.
   */
  const login = useCallback(
    async (email: string, password: string) => {
      setLoading(true);
      setError(null);
      try {
        const data = await api.post<{ user: User }>('/auth/login', {
          email,
          password,
        });
        setUser(data.user);
      } catch (err: any) {
        setUser(null);
        setError(err?.message ?? 'Login failed');
        // Re‑throw so UI components can react (e.g., form error handling)
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  /**
   * Logout flow.
   */
  const logout = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      await api.post('/auth/logout', {});
      setUser(null);
    } catch (err: any) {
      setError(err?.message ?? 'Logout failed');
      throw err;
    } finally {
      setLoading(false);
    }
  }, []);

  /**
   * Initialise auth state on component mount.
   */
  useEffect(() => {
    // Initial session validation
    refresh();
  }, [refresh]);

  /**
   * Periodic token refresh.
   * The backend typically issues short‑lived JWTs (e.g., 15 min).
   * Refreshing every 5 min keeps the session alive without user interaction.
   */
  useEffect(() => {
    const interval = setInterval(() => {
      refresh();
    }, 5 * 60 * 1000); // 5 minutes

    return () => clearInterval(interval);
  }, [refresh]);

  const value: AuthContextValue = {
    user,
    loading,
    error,
    login,
    logout,
    refresh,
    isAuthenticated: !!user,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

/**
 * Hook for consuming the auth context.
 *
 * Must be used within an <AuthProvider>.
 */
export const useAuth = (): AuthContextValue => {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};