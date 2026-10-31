import React, { useEffect, useRef, ReactNode } from "react";
import ReactDOM from "react-dom";
import FocusTrap from "focus-trap-react";

export interface ModalProps {
  /** Controls the visibility of the modal */
  isOpen: boolean;
  /** Callback invoked when the modal requests to be closed (overlay click, ESC, close button) */
  onClose: () => void;
  /** Optional title displayed at the top of the modal */
  title?: string;
  /** Modal content */
  children: ReactNode;
  /** Size variant – controls the max‑width of the modal */
  size?: "sm" | "md" | "lg";
  /** Additional class names for the modal container */
  className?: string;
}

/**
 * Generic, accessible modal component.
 *
 * - Renders via a portal to `document.body`.
 * - Uses `focus-trap-react` to keep keyboard focus inside while open.
 * - Closes on overlay click, Escape key, or close button.
 * - Restores body scroll when closed.
 * - Fully styled with Tailwind CSS and respects the project's design system.
 */
export const Modal: React.FC<ModalProps> = ({
  isOpen,
  onClose,
  title,
  children,
  size = "md",
  className = "",
}) => {
  const overlayRef = useRef<HTMLDivElement>(null);
  const previouslyFocusedElement = useRef<Element | null>(null);

  // Lock body scroll when modal is open
  useEffect(() => {
    if (isOpen) {
      previouslyFocusedElement.current = document.activeElement;
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
      // Return focus to the element that opened the modal
      if (previouslyFocusedElement.current instanceof HTMLElement) {
        previouslyFocusedElement.current.focus();
      }
    }

    return () => {
      document.body.style.overflow = "";
    };
  }, [isOpen]);

  // Close on Escape key
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.stopPropagation();
        onClose();
      }
    };
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, onClose]);

  // Close when clicking on the overlay (but not when clicking inside the modal)
  const handleOverlayClick = (e: React.MouseEvent) => {
    if (e.target === overlayRef.current) {
      onClose();
    }
  };

  if (!isOpen) {
    return null;
  }

  const sizeClasses = {
    sm: "max-w-sm",
    md: "max-w-md",
    lg: "max-w-lg",
  }[size];

  const modalContent = (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 backdrop-blur-sm"
      aria-modal="true"
      role="dialog"
      aria-labelledby={title ? "modal-title" : undefined}
      onMouseDown={handleOverlayClick}
    >
      <FocusTrap
        focusTrapOptions={{
          // Return focus to the overlay when deactivated (handled manually above)
          clickOutsideDeactivates: true,
          escapeDeactivates: false,
        }}
      >
        <div
          className={`relative w-full ${sizeClasses} bg-white dark:bg-gray-800 rounded-lg shadow-xl transform transition-all ${className}`}
          role="document"
        >
          {/* Header */}
          {title && (
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h2 id="modal-title" className="text-lg font-medium text-gray-900 dark:text-gray-100">
                {title}
              </h2>
              <button
                type="button"
                onClick={onClose}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 focus:outline-none focus:ring-2 focus:ring-primary-500 rounded"
                aria-label="Close modal"
              >
                <svg
                  className="h-5 w-5"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  aria-hidden="true"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          )}

          {/* Body */}
          <div className="px-6 py-4 overflow-y-auto max-h-[80vh]">{children}</div>
        </div>
      </FocusTrap>
    </div>
  );

  return ReactDOM.createPortal(modalContent, document.body);
};

export default Modal;