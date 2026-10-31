import React, {
  useState,
  useRef,
  useCallback,
  ChangeEvent,
  DragEvent,
  useEffect,
} from 'react';
import { api } from '../../utils/api';

type InvoiceUploadProps = {
  /** Called after a successful upload so the parent can refresh data */
  onUploadSuccess?: () => void;
};

const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10 MiB
const ACCEPTED_TYPES = [
  'image/jpeg',
  'image/png',
  'image/webp',
  'application/pdf',
];

export const InvoiceUpload: React.FC<InvoiceUploadProps> = ({
  onUploadSuccess,
}) => {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [previews, setPreviews] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);

  // -------------------------------------------------------------------------
  // Helpers
  // -------------------------------------------------------------------------
  const validateFile = (file: File): string | null => {
    if (!ACCEPTED_TYPES.includes(file.type)) {
      return `Unsupported file type: ${file.type}`;
    }
    if (file.size > MAX_FILE_SIZE) {
      return `File “${file.name}” exceeds the ${MAX_FILE_SIZE / (1024 * 1024)} MiB limit`;
    }
    return null;
  };

  const createPreview = (file: File): string => {
    // PDFs get a generic icon; images get a blob URL.
    if (file.type === 'application/pdf') {
      return '/pdf-icon.svg';
    }
    return URL.createObjectURL(file);
  };

  const revokePreviews = () => {
    previews.forEach((url) => {
      // Only revoke blob URLs (ignore the static PDF icon)
      if (url.startsWith('blob:')) {
        URL.revokeObjectURL(url);
      }
    });
  };

  // -------------------------------------------------------------------------
  // File handling
  // -------------------------------------------------------------------------
  const addFiles = useCallback(
    (fileList: FileList) => {
      const newFiles: File[] = [];
      const newPreviews: string[] = [];
      for (let i = 0; i < fileList.length; i++) {
        const file = fileList[i];
        const validationError = validateFile(file);
        if (validationError) {
          setError(validationError);
          continue;
        }
        newFiles.push(file);
        newPreviews.push(createPreview(file));
      }
      if (newFiles.length) {
        setFiles((prev) => [...prev, ...newFiles]);
        setPreviews((prev) => [...prev, ...newPreviews]);
        setError(null);
      }
    },
    [previews],
  );

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      addFiles(e.dataTransfer.files);
      e.dataTransfer.clearData();
    }
  };

  const handleDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleFileSelect = (e: ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      addFiles(e.target.files);
    }
  };

  const openFileDialog = () => {
    fileInputRef.current?.click();
  };

  const removeFile = (index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
    setPreviews((prev) => {
      const url = prev[index];
      if (url.startsWith('blob:')) {
        URL.revokeObjectURL(url);
      }
      return prev.filter((_, i) => i !== index);
    });
  };

  // -------------------------------------------------------------------------
  // Upload
  // -------------------------------------------------------------------------
  const upload = async () => {
    if (files.length === 0) {
      setError('Select at least one file before uploading.');
      return;
    }

    setUploading(true);
    setProgress(0);
    setError(null);

    const formData = new FormData();
    files.forEach((file) => {
      formData.append('files', file);
    });

    // Use XMLHttpRequest to get progress events (fetch lacks them)
    const xhr = new XMLHttpRequest();
    xhr.open('POST', api('/api/invoices'));
    xhr.withCredentials = true; // send HttpOnly JWT cookie

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const percent = Math.round((e.loaded / e.total) * 100);
        setProgress(percent);
      }
    };

    xhr.onload = () => {
      setUploading(false);
      if (xhr.status >= 200 && xhr.status < 300) {
        // Success – reset state
        revokePreviews();
        setFiles([]);
        setPreviews([]);
        setProgress(0);
        onUploadSuccess?.();
      } else {
        setError(
          `Upload failed (${xhr.status}): ${xhr.statusText || 'unknown error'}`,
        );
      }
    };

    xhr.onerror = () => {
      setUploading(false);
      setError('Network error – could not complete the upload.');
    };

    xhr.send(formData);
  };

  // -------------------------------------------------------------------------
  // Cleanup
  // -------------------------------------------------------------------------
  useEffect(() => {
    return () => {
      revokePreviews();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // -------------------------------------------------------------------------
  // Render
  // -------------------------------------------------------------------------
  return (
    <div className="w-full max-w-2xl mx-auto">
      {/* Dropzone */}
      <div
        className={`
          border-2 border-dashed rounded-lg p-8 text-center cursor-pointer
          transition-colors
          ${uploading ? 'opacity-50 cursor-not-allowed' : ''}
          hover:border-primary focus:outline-none focus:ring-2 focus:ring-primary
        `}
        onClick={uploading ? undefined : openFileDialog}
        onDrop={uploading ? undefined : handleDrop}
        onDragOver={uploading ? undefined : handleDragOver}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (!uploading && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            openFileDialog();
          }
        }}
        aria-label="Upload invoice files"
      >
        <input
          type="file"
          multiple
          accept={ACCEPTED_TYPES.join(',')}
          className="hidden"
          ref={fileInputRef}
          onChange={handleFileSelect}
          disabled={uploading}
        />
        <svg
          className="mx-auto h-12 w-12 text-gray-400"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 48 48"
          aria-hidden="true"
        >
          <path
            d="M28 8H12a2 2 0 00-2 2v28a2 2 0 002 2h24a2 2 0 002-2V16l-8-8z"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M28 8v8h8"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">
          Drag &amp; drop invoices here, or{' '}
          <span className="text-primary font-medium underline">browse</span>
        </p>
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
          Accepted: JPEG, PNG, WebP, PDF – up to {MAX_FILE_SIZE / (1024 * 1024)} MiB
        </p>
      </div>

      {/* File preview list */}
      {files.length > 0 && (
        <div className="mt-6 grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
          {files.map((file, idx) => (
            <div
              key={idx}
              className="relative rounded-md overflow-hidden border border-gray-200 dark:border-gray-700"
            >
              {previews[idx].endsWith('.svg') ? (
                <img
                  src={previews[idx]}
                  alt="PDF icon"
                  className="object-cover w-full h-32 bg-gray-100 dark:bg-gray-800"
                />
              ) : (
                <img
                  src={previews[idx]}
                  alt={file.name}
                  className="object-cover w-full h-32"
                />
              )}
              <button
                type="button"
                onClick={() => removeFile(idx)}
                className="absolute top-1 right-1 bg-white/75 dark:bg-gray-800/75 rounded-full p-1 hover:bg-white dark:hover:bg-gray-800 focus:outline-none focus:ring-2 focus:ring-primary"
                aria-label={`Remove ${file.name}`}
              >
                <svg
                  className="h-4 w-4 text-gray-600 dark:text-gray-300"
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <path
                    fillRule="evenodd"
                    d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                    clipRule="evenodd"
                  />
                </svg>
              </button>
              <div className="absolute bottom-0 left-0 right-0 bg-black/30 text-white text-xs px-1 py-0.5 truncate">
                {file.name}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Upload button & progress */}
      <div className="mt-6 flex items-center space-x-4">
        <button
          type="button"
          onClick={upload}
          disabled={uploading || files.length === 0}
          className={`
            px-4 py-2 bg-primary text-white rounded-md
            hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-primary
            disabled:opacity-50 disabled:cursor-not-allowed
          `}
        >
          {uploading ? (
            <span className="flex items-center">
              <svg
                className="animate-spin h-5 w-5 mr-2 text-white"
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
                  d="M4 12a8 8 0 018-8v8H4z"
                />
              </svg>
              Uploading…
            </span>
          ) : (
            'Upload'
          )}
        </button>

        {uploading && (
          <div className="flex-1">
            <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5 overflow-hidden">
              <div
                className="bg-primary h-full transition-width duration-300"
                style={{ width: `${progress}%` }}
              />
            </div>
            <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
              {progress}% completed
            </p>
          </div>
        )}
      </div>

      {/* Error message */}
      {error && (
        <p className="mt-4 text-sm text-danger">{error}</p>
      )}
    </div>
  );
};

export default InvoiceUpload;