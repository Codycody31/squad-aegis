<script setup lang="ts">
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ref } from "vue";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormDescription,
  FormMessage,
} from "@/components/ui/form";
import { toast } from "~/components/ui/toast";

const runtimeConfig = useRuntimeConfig();
const loginError = ref<string | null>(null);

useHead({
  title: "Login",
});

definePageMeta({
  layout: "blank",
  middleware: "guest",
});

const formSchema = toTypedSchema(
  z.object({
    username: z.string().min(1, "Username is required"),
    password: z.string().min(1, "Password is required"),
  })
);

const form = useForm({
  validationSchema: formSchema,
});

const onSubmit = form.handleSubmit(async (values) => {
  loginError.value = null;

  const { data, error } = await useFetch(
    `${runtimeConfig.public.backendApi}/auth/login`,
    {
      method: "POST",
      body: values,
    }
  );

  if (error.value) {
    const errorMessage = error.value.data?.message || "Invalid username or password";
    loginError.value = errorMessage;
    toast({
      title: "Login Failed",
      description: errorMessage,
      variant: "destructive",
    });
    return;
  }

  if (data.value) {
    let { token, expires_at } = data.value.data.session;
    expires_at = new Date(expires_at);
    document.cookie = `${runtimeConfig.public.sessionCookieName}=${token}; expires=${expires_at.toUTCString()}; path=/`;
    useAuthStore().fetch();
    navigateTo("/dashboard");
  }
});
</script>

<template>
  <div
    class="flex min-h-svh flex-col items-center justify-center bg-muted p-6 md:p-10"
  >
    <div class="w-full max-w-sm md:max-w-3xl">
      <div class="flex flex-col gap-6">
        <Card class="overflow-hidden">
          <CardContent class="grid p-0 md:grid-cols-2">
            <form class="p-6 md:p-8" @submit="onSubmit">
              <div class="flex flex-col gap-6">
                <div class="flex flex-col items-center text-center">
                  <h1 class="text-2xl font-bold">Welcome back</h1>
                  <p class="text-balance text-muted-foreground">
                    Login to your Squad Aegis account
                  </p>
                </div>
                <div
                  v-if="loginError"
                  class="bg-destructive/15 text-destructive text-sm p-3 rounded-md border border-destructive/30"
                >
                  {{ loginError }}
                </div>
                <div class="grid gap-2">
                  <FormField v-slot="{ componentField }" name="username">
                    <FormItem>
                      <FormLabel>Username</FormLabel>
                      <FormControl>
                        <Input
                          type="text"
                          placeholder="aegis"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                </div>
                <div class="grid gap-2">
                  <FormField v-slot="{ componentField }" name="password">
                    <FormItem>
                      <FormLabel>Password</FormLabel>
                      <FormControl>
                        <Input
                          type="password"
                          placeholder="********"
                          v-bind="componentField"
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  </FormField>
                </div>
                <Button type="submit" class="w-full"> Login </Button>
              </div>
            </form>
            <div class="relative hidden bg-muted md:block">
              <img
                src="/assets/images/squad-logo.png"
                alt="Image"
                class="absolute inset-0 h-full w-full object-contain dark:brightness-[0.2] dark:grayscale mx-auto my-auto"
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  </div>
</template>
