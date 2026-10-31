import { NextPage } from 'next';
import Head from 'next/head';
import { useRouter } from 'next/router';
import { useEffect, useRef, useState, FormEvent } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/utils/api';

const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const LoginPage: NextPage = () => {
  const router = useRouter();
  const { login, user, loading: authLoading } = useAuth();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; password?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const emailRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);

  // Redirect if already authenticated
  useEffect(() => {
    if (user && !authLoading) {
      router.replace('/');
    }
  }, [user, authLoading, router]);

  const validate = (): boolean => {
    const errors: { email?: string; password?: string } = {};

    if (!email) {
      errors.email = 'Email is required';
    } else if (!emailRegex.test(email)) {
      errors.email = 'Enter a valid email address';
    }

    if (!password) {
      errors.password = 'Password is required';
    } else if (password.length < 8) {
      errors.password = 'Password must be at least 8 characters';
    }

    setFieldErrors(errors);
    // Focus first invalid field
    if (errors.email) {
      emailRef.current?.focus();
    } else if (errors.password) {
      passwordRef.current?.focus();
    }

    return Object.keys(errors).length === 0;
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setFormError(null);

    if (!validate()) {
      return;
    }

    setSubmitting(true);
    try {
      // The API wrapper automatically includes credentials (cookies)
      await api.post('/api/auth/login', { email, password });
      // useAuth hook will refresh the user state via a background call or token check
      await login(email, password);
      router.replace('/');
    } catch (err: any) {
      // Expect API to return { message: string }
      const msg = err?.response?.data?.message ?? 'Unable to login. Please try again.';
      setFormError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Head>
        <title>Login – FinSight AI</title>
      </Head>

      <div className="flex min-h-screen items-center justify-center bg-gray-50 dark:bg-gray-900 px-4">
        <div className="w-full max-w-md space-y-8">
          <div className="text-center">
            <h2 className="mt-6 text-3xl font-semibold text-gray-900 dark:text-gray-100">
              Sign in to your account
            </h2>
            <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
              Access your invoices and spend insights.
            </p>
          </div>

          <form className="mt-8 space-y-6" onSubmit={handleSubmit} noValidate>
            {formError && (
              <div
                role="alert"
                className="rounded-md bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900 dark:text-red-200"
              >
                {formError}
              </div>
            )}

            <div className="rounded-md shadow-sm -space-y-px">
              <div>
                <label htmlFor="email-address" className="sr-only">
                  Email address
                </label>
                <input
                  id="email-address"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  ref={emailRef}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className={`relative block w-full appearance-none rounded-t-md border px-3 py-2 placeholder-gray-500 focus:z-10 focus:border-primary-600 focus:outline-none focus:ring-primary-600 sm:text-sm ${
                    fieldErrors.email
                      ? 'border-red-500 focus:border-red-600 focus:ring-red-600'
                      : 'border-gray-300 dark:border-gray-600'
                  }`}
                  placeholder="Email address"
                  aria-invalid={!!fieldErrors.email}
                  aria-describedby={fieldErrors.email ? 'email-error' : undefined}
                />
                {fieldErrors.email && (
                  <p className="mt-1 text-xs text-red-600 dark:text-red-400" id="email-error">
                    {fieldErrors.email}
                  </p>
                )}
              </div>

              <div className="-mt-px">
                <label htmlFor="password" className="sr-only">
                  Password
                </label>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  required
                  ref={passwordRef}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className={`relative block w-full appearance-none rounded-b-md border px-3 py-2 placeholder-gray-500 focus:z-10 focus:border-primary-600 focus:outline-none focus:ring-primary-600 sm:text-sm ${
                    fieldErrors.password
                      ? 'border-red-500 focus:border-red-600 focus:ring-red-600'
                      : 'border-gray-300 dark:border-gray-600'
                  }`}
                  placeholder="Password"
                  aria-invalid={!!fieldErrors.password}
                  aria-describedby={fieldErrors.password ? 'password-error' : undefined}
                />
                {fieldErrors.password && (
                  <p className="mt-1 text-xs text-red-600 dark:text-red-400" id="password-error">
                    {fieldErrors.password}
                  </p>
                )}
              </div>
            </div>

            <div>
              <button
                type="submit"
                disabled={submitting}
                className={`group relative flex w-full justify-center rounded-md border border-transparent bg-primary-600 py-2 px-4 text-sm font-medium text-white hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed`}
              >
                {submitting ? (
                  <svg
                    className="animate-spin h-5 w-5 text-white mr-2"
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      className="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      strokeWidth="4"
                    />
                    <path
                      className="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8v4l3-3-3-3v4a8 8 0 00-8 8h4z"
                    />
                  </svg>
                ) : null}
                Sign in
              </button>
            </div>
          </form>
        </div>
      </div>
    </>
  );
};

export default LoginPage;