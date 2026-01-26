/**
 * Checks if the current connection is secure (HTTPS) or local (localhost/127.0.0.1)
 * @returns true if the connection is secure or local, false otherwise
 */
export function isSecureOrLocalConnection(): boolean {
  if (typeof window === 'undefined') {
    return false;
  }

  const { protocol, hostname } = window.location;

  // Check if connection is HTTPS
  const isSecure = protocol === 'https:';

  // Check if connection is local
  const isLocal =
    hostname === 'localhost' ||
    hostname === '127.0.0.1' ||
    hostname === '[::1]' || // IPv6 localhost
    hostname.indexOf('192.168.') === 0 || // Local network
    hostname.indexOf('10.') === 0 || // Local network
    /^172\.(1[6-9]|2[0-9]|3[0-1])\./.test(hostname); // Local network (172.16.0.0 - 172.31.255.255)

  return isSecure || isLocal;
}
