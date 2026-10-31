/**
 * frontend/src/utils/validation.ts
 *
 * Centralised client‑side validation helpers used across the application.
 * - Email validation (RFC‑5322‑compatible simple regex)
 * - Password strength validation (length, case, numeric, special character)
 * - File validation (type whitelist & size limit)
 *
 * All helpers return plain values (boolean or error messages) so they can be
 * easily consumed by React components, form libraries (react‑hook‑form, Formik)
 * or custom hooks without pulling in heavy dependencies.
 */

type PasswordValidationResult = {
  /** True when the password satisfies all strength requirements */
  valid: boolean;
  /** Human‑readable list of unmet criteria */
  errors: string[];
};

/**
 * Validate an e‑mail address.
 *
 * Uses a reasonably strict regex that covers the majority of valid addresses
 * while remaining performant for client‑side checks.
 *
 * @param email - The e‑mail string to validate.
 * @returns true if the e‑mail is syntactically valid, false otherwise.
 */
export function isValidEmail(email: string | undefined | null): boolean {
  if (!email) return false;
  // Trim whitespace and enforce lower‑case for consistency
  const trimmed = email.trim().toLowerCase();

  // Simple RFC‑5322‑compatible regex (does not allow quoted strings)
  const emailRegex =
    /^[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)*\.[a-z]{2,}$/i;

  return emailRegex.test(trimmed);
}

/**
 * Validate password strength.
 *
 * Requirements (configurable via constants):
 *   - Minimum length (default 8)
 *   - At least one uppercase letter
 *   - At least one lowercase letter
 *   - At least one digit
 *   - At least one special character (non‑alphanumeric)
 *
 * @param password - The password string to validate.
 * @returns An object describing overall validity and any unmet criteria.
 */
export function validatePassword(password: string | undefined | null): PasswordValidationResult {
  const MIN_LENGTH = 8;

  const errors: string[] = [];

  if (!password) {
    errors.push('Password is required.');
    return { valid: false, errors };
  }

  if (password.length < MIN_LENGTH) {
    errors.push(`Password must be at least ${MIN_LENGTH} characters long.`);
  }

  if (!/[a-z]/.test(password)) {
    errors.push('Password must contain at least one lowercase letter.');
  }

  if (!/[A-Z]/.test(password)) {
    errors.push('Password must contain at least one uppercase letter.');
  }

  if (!/[0-9]/.test(password)) {
    errors.push('Password must contain at least one digit.');
  }

  // Special characters: any non‑alphanumeric Unicode character
  if (!/[^A-Za-z0-9]/.test(password)) {
    errors.push('Password must contain at least one special character.');
  }

  return { valid: errors.length === 0, errors };
}

/**
 * Human‑readable byte formatter (e.g. 1024 → "1 KB").
 *
 * @param bytes - Number of bytes.
 * @returns Formatted string with appropriate unit.
 */
export function formatBytes(bytes: number): string {
  if (bytes < 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  let value = bytes;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${Number(value.toFixed(2))} ${units[i]}`;
}

/**
 * Validate a file against allowed MIME types and maximum size.
 *
 * @param file - The File object from an <input type="file"> or drag‑and‑drop.
 * @param options.allowedTypes - Array of MIME types that are acceptable.
 * @param options.maxSizeBytes - Maximum file size in bytes.
 *
 * @returns An object indicating validity and an optional error message.
 */
export function validateFile(
  file: File | undefined | null,
  options: {
    allowedTypes?: string[];
    maxSizeBytes?: number;
  } = {}
): { valid: boolean; error?: string } {
  if (!file) {
    return { valid: false, error: 'No file selected.' };
  }

  const {
    allowedTypes = ['image/jpeg', 'image/png', 'application/pdf'],
    maxSizeBytes = 5 * 1024 * 1024, // 5 MiB default
  } = options;

  // MIME type validation
  if (!allowedTypes.includes(file.type)) {
    const allowedList = allowedTypes.map((t) => t.split('/')[1].toUpperCase()).join(', ');
    return {
      valid: false,
      error: `Invalid file type. Allowed types: ${allowedList}.`,
    };
  }

  // Size validation
  if (file.size > maxSizeBytes) {
    return {
      valid: false,
      error: `File is too large. Maximum size is ${formatBytes(maxSizeBytes)}.`,
    };
  }

  return { valid: true };
}

/**
 * Convenience wrapper for validating multiple files (e.g. batch uploads).
 *
 * @param files - FileList or array of File objects.
 * @param options - Same options as `validateFile`.
 *
 * @returns An array of validation results aligned with the input order.
 */
export function validateFiles(
  files: FileList | File[],
  options?: {
    allowedTypes?: string[];
    maxSizeBytes?: number;
  }
): Array<{ file: File; valid: boolean; error?: string }> {
  const fileArray = Array.from(files);
  return fileArray.map((file) => ({
    file,
    ...validateFile(file, options),
  }));
}