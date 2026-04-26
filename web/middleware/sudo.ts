export default defineNuxtRouteMiddleware(() => {
  const authStore = useAuthStore();

  if (!authStore.user?.super_admin) {
    return navigateTo("/dashboard");
  }
});
