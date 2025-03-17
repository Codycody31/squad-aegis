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