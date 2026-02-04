/**
 * Format a date as a localized date and time string
 */
export function dateTimeFormat(date: Date): string {
  return date.toLocaleString();
}

/**
 * Format a date as a localized date string (without time)
 */
export function dateFormat(date: Date): string {
  return date.toLocaleDateString();
}

/**
 * Format a date as a localized time string (without date)
 */
export function timeFormat(date: Date): string {
  return date.toLocaleTimeString();
}

/**
 * Format a date relative to now (e.g., "2 hours ago")
 */
export function relativeTimeFormat(date: Date): string {
  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);
  
  if (diffInSeconds < 60) {
    return 'just now';
  }
  
  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) {
    return `${diffInMinutes} minute${diffInMinutes === 1 ? '' : 's'} ago`;
  }
  
  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) {
    return `${diffInHours} hour${diffInHours === 1 ? '' : 's'} ago`;
  }
  
  const diffInDays = Math.floor(diffInHours / 24);
  if (diffInDays < 30) {
    return `${diffInDays} day${diffInDays === 1 ? '' : 's'} ago`;
  }
  
  const diffInMonths = Math.floor(diffInDays / 30);
  if (diffInMonths < 12) {
    return `${diffInMonths} month${diffInMonths === 1 ? '' : 's'} ago`;
  }
  
  const diffInYears = Math.floor(diffInMonths / 12);
  return `${diffInYears} year${diffInYears === 1 ? '' : 's'} ago`;
}

/**
 * Format a duration in seconds to a human-readable string
 * e.g., "2h 15m", "45m", "1d 3h"
 */
export function formatSessionDuration(seconds: number): string {
  if (seconds < 0) return "0s";
  if (seconds < 60) return `${Math.floor(seconds)}s`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (hours < 24) {
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  }

  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`;
} 