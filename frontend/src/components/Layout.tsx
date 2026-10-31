import React, { useState, useEffect, ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/router";
import { useAuth } from "@/hooks/useAuth";
import {
  SunIcon,
  MoonIcon,
  Bars3Icon,
  XMarkIcon,
  HomeIcon,
  DocumentDuplicateIcon,
  Cog6ToothIcon,
  ArrowRightOnRectangleIcon,
} from "@heroicons/react/24/outline";

type NavItem = {
  name: string;
  href: string;
  icon: React.ReactNode;
};

const NAV_ITEMS: NavItem[] = [
  {
    name: "Dashboard",
    href: "/",
    icon: <HomeIcon className="h-5 w-5" aria-hidden="true" />,
  },
  {
    name: "Invoices",
    href: "/invoices",
    icon: (
      <DocumentDuplicateIcon className="h-5 w-5" aria-hidden="true" />
    ),
  },
  {
    name: "Settings",
    href: "/settings",
    icon: <Cog6ToothIcon className="h-5 w-5" aria-hidden="true" />,
  },
];

const ThemeToggle: React.FC = () => {
  const [isDark, setIsDark] = useState<boolean>(false);

  // Initialise theme from localStorage / system preference
  useEffect(() => {
    const stored = localStorage.getItem("theme");
    if (stored) {
      setIsDark(stored === "dark");
      document.documentElement.classList.toggle("dark", stored === "dark");
    } else {
      const prefersDark = window.matchMedia(
        "(prefers-color-scheme: dark)"
      ).matches;
      setIsDark(prefersDark);
      document.documentElement.classList.toggle("dark", prefersDark);
    }
  }, []);

  const toggle = () => {
    const newTheme = !isDark;
    setIsDark(newTheme);
    document.documentElement.classList.toggle("dark", newTheme);
    localStorage.setItem("theme", newTheme ? "dark" : "light");
    // In a real implementation you would also persist the preference via an API.
  };

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label="Toggle theme"
      className="rounded-full p-2 text-gray-600 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
    >
      {isDark ? (
        <SunIcon className="h-5 w-5" aria-hidden="true" />
      ) : (
        <MoonIcon className="h-5 w-5" aria-hidden="true" />
      )}
    </button>
  );
};

type LayoutProps = {
  children: ReactNode;
};

export const Layout: React.FC<LayoutProps> = ({ children }) => {
  const router = useRouter();
  const { user, logout, loading: authLoading } = useAuth();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  const handleLogout = async () => {
    try {
      await logout();
      router.push("/login");
    } catch (err) {
      console.error("Logout failed:", err);
      // Optionally show a toast/notification here.
    }
  };

  const isActive = (href: string) => router.pathname === href;

  return (
    <div className="flex h-screen bg-background text-primary">
      {/* Sidebar */}
      <aside className="hidden md:flex md:w-64 md:flex-col md:border-r md:border-gray-200 md:bg-surface">
        <div className="flex h-16 items-center justify-between px-4 border-b border-gray-200">
          <Link href="/" className="text-lg font-semibold text-primary">
            FinSight AI
          </Link>
          <ThemeToggle />
        </div>
        <nav className="flex-1 overflow-y-auto py-4">
          <ul className="space-y-1">
            {NAV_ITEMS.map((item) => (
              <li key={item.name}>
                <Link href={item.href}>
                  <a
                    className={`flex items-center px-4 py-2 text-sm font-medium rounded-md focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
                      isActive(item.href)
                        ? "bg-primary-100 text-primary"
                        : "text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
                    }`}
                  >
                    {item.icon}
                    <span className="ml-3">{item.name}</span>
                  </a>
                </Link>
              </li>
            ))}
          </ul>
        </nav>
        <div className="border-t border-gray-200 p-4">
          {user && (
            <div className="flex items-center space-x-3">
              <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary-500 text-white">
                {user.email?.charAt(0).toUpperCase() ?? "U"}
              </div>
              <div className="flex-1 truncate text-sm font-medium">
                {user.email}
              </div>
              <button
                type="button"
                onClick={handleLogout}
                disabled={authLoading}
                className="rounded-full p-2 text-gray-600 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                aria-label="Logout"
              >
                <ArrowRightOnRectangleIcon
                  className="h-5 w-5"
                  aria-hidden="true"
                />
              </button>
            </div>
          )}
        </div>
      </aside>

      {/* Mobile navigation */}
      <div className={`fixed inset-0 z-40 md:hidden ${mobileNavOpen ? "" : "pointer-events-none"}`}>
        <div
          className={`fixed inset-0 bg-black bg-opacity-25 transition-opacity ${mobileNavOpen ? "opacity-100" : "opacity-0"
            }`}
          aria-hidden="true"
          onClick={() => setMobileNavOpen(false)}
        />
        <aside
          className={`fixed inset-y-0 left-0 w-64 bg-surface shadow-xl transform transition-transform ${mobileNavOpen ? "translate-x-0" : "-translate-x-full"
            }`}
        >
          <div className="flex h-16 items-center justify-between px-4 border-b border-gray-200">
            <Link href="/" className="text-lg font-semibold text-primary">
              FinSight AI
            </Link>
            <button
              type="button"
              onClick={() => setMobileNavOpen(false)}
              className="rounded-md p-2 text-gray-600 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
              aria-label="Close navigation"
            >
              <XMarkIcon className="h-6 w-6" aria-hidden="true" />
            </button>
          </div>
          <nav className="flex-1 overflow-y-auto py-4">
            <ul className="space-y-1">
              {NAV_ITEMS.map((item) => (
                <li key={item.name}>
                  <Link href={item.href}>
                    <a
                      className={`flex items-center px-4 py-2 text-sm font-medium rounded-md focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
                        isActive(item.href)
                          ? "bg-primary-100 text-primary"
                          : "text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700"
                      }`}
                      onClick={() => setMobileNavOpen(false)}
                    >
                      {item.icon}
                      <span className="ml-3">{item.name}</span>
                    </a>
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
          <div className="border-t border-gray-200 p-4">
            {user && (
              <div className="flex items-center space-x-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary-500 text-white">
                  {user.email?.charAt(0).toUpperCase() ?? "U"}
                </div>
                <div className="flex-1 truncate text-sm font-medium">
                  {user.email}
                </div>
                <button
                  type="button"
                  onClick={handleLogout}
                  disabled={authLoading}
                  className="rounded-full p-2 text-gray-600 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                  aria-label="Logout"
                >
                  <ArrowRightOnRectangleIcon
                    className="h-5 w-5"
                    aria-hidden="true"
                  />
                </button>
              </div>
            )}
          </div>
        </aside>
      </div>

      {/* Main content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top bar */}
        <header className="flex h-16 items-center justify-between border-b border-gray-200 bg-surface px-4 md:px-6">
          <button
            type="button"
            className="md:hidden rounded-md p-2 text-gray-600 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            onClick={() => setMobileNavOpen(true)}
            aria-label="Open navigation"
          >
            <Bars3Icon className="h-6 w-6" aria-hidden="true" />
          </button>
          <div className="flex items-center space-x-4">
            <ThemeToggle />
            {user && (
              <div className="flex items-center space-x-2">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-200">
                  {user.email}
                </span>
                <button
                  type="button"
                  onClick={handleLogout}
                  disabled={authLoading}
                  className="rounded-full p-2 text-gray-600 hover:bg-gray-200 dark:text-gray-300 dark:hover:bg-gray-700 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                  aria-label="Logout"
                >
                  <ArrowRightOnRectangleIcon
                    className="h-5 w-5"
                    aria-hidden="true"
                  />
                </button>
              </div>
            )}
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-4 md:p-6">{children}</main>
      </div>
    </div>
  );
};

export default Layout;