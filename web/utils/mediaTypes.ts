export type MediaCategory = "image" | "video" | "pdf" | "text" | "unknown";

export function getMediaCategory(mimeType: string | null | undefined): MediaCategory {
  if (!mimeType) return "unknown";

  const type = mimeType.toLowerCase();
  if (type.startsWith("image/")) return "image";
  if (type.startsWith("video/")) return "video";
  if (type === "application/pdf") return "pdf";
  if (type === "text/plain") return "text";
  return "unknown";
}

export function canPreview(mimeType: string | null | undefined): boolean {
  return getMediaCategory(mimeType) !== "unknown";
}

export const SUPPORTED_PREVIEW_TYPES = {
  image: ["image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp"],
  video: ["video/mp4", "video/webm", "video/quicktime"],
  pdf: ["application/pdf"],
  text: ["text/plain"],
} as const;
