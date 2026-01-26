import type { UseFetchOptions } from 'nuxt/app'
import { toast } from '~/components/ui/toast'

/**
 * Custom fetch composable that handles session expiration (401) globally
 * Automatically redirects to login and clears session when unauthorized
 */
export function useAuthFetch<T>(url: string, options?: UseFetchOptions<T>) {
  const runtimeConfig = useRuntimeConfig()
  const authStore = useAuthStore()
  const sessionCookie = useCookie(runtimeConfig.public.sessionCookieName as string)

  // Merge headers properly - Authorization should always be set
  const mergedHeaders = {
    Authorization: `Bearer ${sessionCookie.value}`,
    ...(options?.headers || {}),
  }

  const defaultOptions: UseFetchOptions<T> = {
    ...options,
    headers: mergedHeaders,
    onResponseError({ response }) {
      // Handle 401 Unauthorized - session expired or invalid
      if (response.status === 401) {
        // Clear session
        authStore.logout()
        sessionCookie.value = null

        // Show user-friendly message
        toast({
          title: 'Session Expired',
          description: 'Your session has expired. Please log in again.',
          variant: 'destructive',
        })

        // Redirect to login
        navigateTo('/login')
      }
    },
  }

  return useFetch<T>(url, defaultOptions)
}

/**
 * Custom $fetch wrapper that handles session expiration (401) globally
 * Use this for imperative API calls (non-reactive)
 */
export async function useAuthFetchImperative<T>(url: string, options?: any): Promise<T> {
  const runtimeConfig = useRuntimeConfig()
  const authStore = useAuthStore()
  const sessionCookie = useCookie(runtimeConfig.public.sessionCookieName as string)

  try {
    return await $fetch<T>(url, {
      headers: {
        Authorization: `Bearer ${sessionCookie.value}`,
        ...options?.headers,
      },
      ...options,
    })
  } catch (error: any) {
    // Handle 401 Unauthorized - session expired or invalid
    if (error?.response?.status === 401 || error?.statusCode === 401) {
      // Clear session
      authStore.logout()
      sessionCookie.value = null

      // Show user-friendly message
      toast({
        title: 'Session Expired',
        description: 'Your session has expired. Please log in again.',
        variant: 'destructive',
      })

      // Redirect to login
      navigateTo('/login')

      // Re-throw with user-friendly message
      throw new Error('Session expired. Please log in again.')
    }

    // Re-throw other errors
    throw error
  }
}
