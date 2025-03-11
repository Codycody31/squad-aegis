export default defineNuxtRouteMiddleware((to) => {
  try {
    if (to.path === "/login") return;

    const runtimeConfig = useRuntimeConfig();
    const cookieToken = useCookie(
      runtimeConfig.public.sessionCookieName as string
    );
    const token = cookieToken.value;

    const redirectToLogin = () => {
      const redirectPath = to.fullPath;

      return navigateTo({
        path: "/login",
        query: { redirect: redirectPath },
      });
    };

    if (!token) return redirectToLogin();
  } catch (e) {
    return navigateTo({ path: "/login" });
  }
});
