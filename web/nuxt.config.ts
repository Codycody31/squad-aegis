// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: "2024-11-01",

  app: {
    buildAssetsDir: "/_static/",
    head: {
      meta: [
        {
          name: "viewport",
          content: "width=device-width, initial-scale=1",
        },
        {
          charset: "utf-8",
        },
      ],
      link: [{ rel: "icon", type: "image/png", href: "/icons/favicon.png" }],
      htmlAttrs: {
        lang: "en",
      },
      charset: "utf-8",
    },
  },

  devtools: {
    enabled: true,

    timeline: {
      enabled: true,
    },
  },

  runtimeConfig: {
    public: {
      sessionCookieName: "session", // can be overridden by NUXT_PUBLIC_SESSION_COOKIE_NAME environment variable
      backendApi: "http://127.0.0.1:3113/api", // can be overridden by NUXT_PUBLIC_BACKEND_API environment variable
      baseUrl: "http://127.0.0.1:3113", // can be overridden by NUXT_PUBLIC_BASE_URL environment variable
    },
  },

  routeRules: {
    "/": {
      redirect: "/dashboard",
    },
  },

  modules: ["@nuxtjs/tailwindcss", "shadcn-nuxt", "@pinia/nuxt", "@nuxt/icon"],

  shadcn: {
    /**
     * Prefix for all the imported component
     */
    prefix: "",
    /**
     * Directory that the component lives in.
     * @default "./components/ui"
     */
    componentDir: "./components/ui",
  },

  pinia: {
    storesDirs: ["./stores/**"],
  },
});
