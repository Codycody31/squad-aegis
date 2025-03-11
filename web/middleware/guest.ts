export default defineNuxtRouteMiddleware(() => {
  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (token) {
    return navigateTo("/dashboard");
  }
});
