import {
  useRef,
  useState,
  useCallback,
  type DragEvent,
  type ChangeEvent,
  type KeyboardEvent,
} from "react";
import { UploadCloudIcon, FileIcon, XIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface FileDropzoneProps {
  accept?: string;
  maxBytes?: number;
  onFileAccepted: (file: File) => void;
  onFileRejected: (reason: string) => void;
  label?: string;
  'data-testid'?: string;
}

function getAllowedExtensions(accept: string): string[] {
  return accept
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.startsWith("."))
    .map((s) => s.toLowerCase());
}

function validateFile(
  file: File,
  accept: string | undefined,
  maxBytes: number | undefined
): string | null {
  if (accept) {
    const extensions = getAllowedExtensions(accept);
    if (extensions.length > 0) {
      const nameLower = file.name.toLowerCase();
      const allowed = extensions.some((ext) => nameLower.endsWith(ext));
      if (!allowed) {
        const extList = extensions.join(", ");
        return `File type not allowed. Expected: ${extList}.`;
      }
    }
  }

  if (maxBytes !== undefined && file.size > maxBytes) {
    const maxMB = (maxBytes / (1024 * 1024)).toFixed(1);
    return `File is too large. Maximum size is ${maxMB} MB.`;
  }

  return null;
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default function FileDropzone({
  accept,
  maxBytes,
  onFileAccepted,
  onFileRejected,
  label = "Drop a file here, or click to browse",
  'data-testid': dataTestId,
}: FileDropzoneProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [isDraggingOver, setIsDraggingOver] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);

  const processFile = useCallback(
    (file: File) => {
      const error = validateFile(file, accept, maxBytes);
      if (error) {
        setSelectedFile(null);
        onFileRejected(error);
        return;
      }
      setSelectedFile(file);
      onFileAccepted(file);
    },
    [accept, maxBytes, onFileAccepted, onFileRejected]
  );

  function handleDragOver(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.stopPropagation();
    setIsDraggingOver(true);
  }

  function handleDragLeave(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.stopPropagation();
    setIsDraggingOver(false);
  }

  function handleDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.stopPropagation();
    setIsDraggingOver(false);
    const file = e.dataTransfer.files[0];
    if (file) processFile(file);
  }

  function handleInputChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (file) processFile(file);
    // Reset so the same file can be re-selected after rejection
    e.target.value = "";
  }

  function handleKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      inputRef.current?.click();
    }
  }

  function handleReplace() {
    setSelectedFile(null);
    inputRef.current?.click();
  }

  const hasFile = selectedFile !== null;

  return (
    <div
      role="button"
      tabIndex={0}
      aria-label={label}
      data-testid={dataTestId}
      onClick={() => !hasFile && inputRef.current?.click()}
      onKeyDown={handleKeyDown}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed px-6 py-8 text-center transition-colors duration-150 outline-none",
        "focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] focus-visible:ring-offset-2",
        isDraggingOver
          ? "border-[var(--tailwind-colors-rdns-600)] bg-[var(--tailwind-colors-rdns-600)]/10 cursor-copy"
          : hasFile
          ? "border-[var(--tailwind-colors-slate-600)] bg-[var(--tailwind-colors-slate-700)]/50 cursor-default"
          : "border-[var(--tailwind-colors-slate-600)] bg-[var(--tailwind-colors-slate-700)]/30 cursor-pointer hover:border-[var(--tailwind-colors-slate-400)] hover:bg-[var(--tailwind-colors-slate-700)]/50"
      )}
    >
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        onChange={handleInputChange}
        className="sr-only"
        tabIndex={-1}
        aria-hidden="true"
      />

      {hasFile ? (
        <>
          <FileIcon className="h-8 w-8 text-[var(--tailwind-colors-rdns-600)] shrink-0" />
          <div className="flex flex-col gap-1 min-w-0 max-w-full">
            <span className="text-sm font-medium text-[var(--tailwind-colors-slate-50)] truncate max-w-[240px]">
              {selectedFile.name}
            </span>
            <span className="text-xs text-[var(--tailwind-colors-slate-400)]">
              {formatBytes(selectedFile.size)}
            </span>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={(e) => {
              e.stopPropagation();
              handleReplace();
            }}
            className="gap-1.5 text-[var(--tailwind-colors-slate-200)] border-[var(--tailwind-colors-slate-600)] hover:border-[var(--tailwind-colors-slate-400)]"
          >
            <XIcon className="h-3.5 w-3.5" />
            Replace
          </Button>
        </>
      ) : (
        <>
          <UploadCloudIcon
            className={cn(
              "h-10 w-10 shrink-0 transition-colors duration-150",
              isDraggingOver
                ? "text-[var(--tailwind-colors-rdns-600)]"
                : "text-[var(--tailwind-colors-slate-400)]"
            )}
          />
          <div className="flex flex-col gap-1">
            <span className="text-sm font-medium text-[var(--tailwind-colors-slate-200)]">
              {label}
            </span>
            {accept && (
              <span className="text-xs text-[var(--tailwind-colors-slate-400)]">
                {getAllowedExtensions(accept).join(", ")} files
                {maxBytes !== undefined && ` up to ${formatBytes(maxBytes)}`}
              </span>
            )}
          </div>
        </>
      )}
    </div>
  );
}
