export default defineNuxtPlugin(() => {
  const authStore = useAuthStore()
  const runtimeConfig = useRuntimeConfig()

  // Global fetch interceptor for handling 401 errors
  const globalFetch = $fetch.create({
    onResponseError({ response }) {
      // Handle 401 Unauthorized - session expired or invalid
      if (response.status === 401) {
        // Clear session
        authStore.logout()
        const sessionCookie = useCookie(runtimeConfig.public.sessionCookieName as string)
        sessionCookie.value = null

        // Show user-friendly message
        const toast = useToast()
        toast.toast({
          title: 'Session Expired',
          description: 'Your session has expired. Please log in again.',
          variant: 'destructive',
        })

        // Redirect to login
        navigateTo('/login')
      }
    },
  })

  return {
    provide: {
      api: globalFetch,
    },
  }
})
