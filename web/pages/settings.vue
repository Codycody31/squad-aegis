<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "~/components/ui/card";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";
import { useForm } from "vee-validate";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";
import type { User } from "@/types";
import { useAuthStore } from "@/stores/auth";

const authStore = useAuthStore();

// Form schemas
const profileFormSchema = toTypedSchema(
  z.object({
    name: z.string().min(1, "Name is required"),
    steamId: z
      .number()
      .optional(),
  })
);

const passwordFormSchema = toTypedSchema(
  z.object({
    currentPassword: z.string().min(1, "Current password is required"),
    newPassword: z
      .string()
      .min(8, "Password must be at least 8 characters")
      .regex(
        /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$/,
        "Password must contain at least one uppercase letter, one lowercase letter, one number, and one special character"
      ),
    confirmPassword: z.string().min(1, "Please confirm your password"),
  }).refine((data) => data.newPassword === data.confirmPassword, {
    message: "Passwords do not match",
    path: ["confirmPassword"],
  })
);

// Setup forms
const profileForm = useForm({
  validationSchema: profileFormSchema,
  initialValues: {
    name: "",
    steamId: undefined,
  },
});

const passwordForm = useForm({
  validationSchema: passwordFormSchema,
  initialValues: {
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
  },
});

// State variables
const user = ref<User | null>(null);
const loading = ref({
  profile: false,
  password: false,
  fetchUser: false,
});
const error = ref<string | null>(null);
const successMessage = ref<string | null>(null);

// Function to show success message with auto-dismiss
function showSuccess(message: string) {
  successMessage.value = message;
  setTimeout(() => {
    successMessage.value = null;
  }, 5000); // Auto-dismiss after 5 seconds
}

// Function to fetch user data
async function fetchUserData() {
  loading.value.fetchUser = true;
  error.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.fetchUser = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch<{ data: { user: User } }>(
      `${runtimeConfig.public.backendApi}/auth/initial`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data?.message || fetchError.value.message || "Failed to fetch user data"
      );
    }

    if (data.value && data.value.data && data.value.data.user) {
      user.value = data.value.data.user;
      console.log(user.value);
      // Update form values
      profileForm.setValues({
        name: user.value.name || "",
        steamId: user.value.steam_id || undefined,
      });
    }
  } catch (err: any) {
    error.value = err.message || "An error occurred while fetching user data";
    console.error(err);
  } finally {
    loading.value.fetchUser = false;
  }
}

// Function to update profile
async function updateProfile(values: any) {
  loading.value.profile = true;
  error.value = null;
  successMessage.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.profile = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/auth/me`,
      {
        method: "PATCH",
        body: {
          name: values.name,
          steamId: values.steamId || null,
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data?.message || fetchError.value.message || "Failed to update profile"
      );
    }

    await authStore.fetch();

    showSuccess("Your profile information has been updated successfully.");

    // Refresh user data
    fetchUserData();
  } catch (err: any) {
    error.value = err.message || "An error occurred while updating profile";
    console.error(err);
  } finally {
    loading.value.profile = false;
  }
}

// Function to change password
async function changePassword(values: any) {
  loading.value.password = true;
  error.value = null;
  successMessage.value = null;

  const runtimeConfig = useRuntimeConfig();
  const cookieToken = useCookie(
    runtimeConfig.public.sessionCookieName as string
  );
  const token = cookieToken.value;

  if (!token) {
    error.value = "Authentication required";
    loading.value.password = false;
    return;
  }

  try {
    const { data, error: fetchError } = await useFetch(
      `${runtimeConfig.public.backendApi}/auth/me/password`,
      {
        method: "PATCH",
        body: {
          currentPassword: values.currentPassword,
          newPassword: values.newPassword,
        },
        headers: {
          Authorization: `Bearer ${token}`,
        },
      }
    );

    if (fetchError.value) {
      throw new Error(
        fetchError.value.data?.message || fetchError.value.message || "Failed to change password"
      );
    }

    showSuccess("Your password has been changed successfully.");

    // Reset password form
    passwordForm.resetForm();
  } catch (err: any) {
    error.value = err.message || "An error occurred while changing password";
    console.error(err);
  } finally {
    loading.value.password = false;
  }
}

// Load user data on mount
onMounted(() => {
  fetchUserData();
});
</script>

<template>
  <div class="p-4">
    <h1 class="text-3xl font-bold mb-8">Account Settings</h1>

    <div v-if="error" class="bg-red-500 text-white p-4 rounded mb-6">
      {{ error }}
    </div>

    <div v-if="successMessage" class="bg-green-500 text-white p-4 rounded mb-6">
      {{ successMessage }}
    </div>

    <div v-if="loading.fetchUser" class="text-center py-8">
      <div
        class="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full mx-auto mb-4"
      ></div>
      <p>Loading your account information...</p>
    </div>

    <div v-else class="grid gap-8">
      <!-- Profile Information -->
      <Card>
        <CardHeader>
          <CardTitle>Profile Information</CardTitle>
          <CardDescription>
            Update your personal information and Steam ID
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form
            v-slot="{ handleSubmit }"
            as="div"
            :validation-schema="profileFormSchema"
            :initial-values="{
              name: user?.name || '',
              steamId: user?.steam_id || '',
            }"
          >
            <form @submit="handleSubmit($event, updateProfile)" class="space-y-4">
              <FormField name="name" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>Name</FormLabel>
                  <FormControl>
                    <Input placeholder="Your name" v-bind="componentField" />
                  </FormControl>
                  <FormDescription>
                    This is your public display name.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              </FormField>

              <FormField name="steamId" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>Steam ID</FormLabel>
                  <FormControl>
                    <Input type="number" placeholder="Your 17-digit Steam ID" v-bind="componentField" />
                  </FormControl>
                  <FormDescription>
                    Your 17-digit Steam ID (SteamID64). This is used to identify you in Squad servers.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              </FormField>

              <Button type="submit" :disabled="loading.profile">
                {{ loading.profile ? "Saving..." : "Save Changes" }}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>

      <!-- Password Change -->
      <Card>
        <CardHeader>
          <CardTitle>Change Password</CardTitle>
          <CardDescription>
            Update your password to keep your account secure
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form
            v-slot="{ handleSubmit }"
            as="div"
            :validation-schema="passwordFormSchema"
          >
            <form @submit="handleSubmit($event, changePassword)" class="space-y-4">
              <FormField name="currentPassword" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>Current Password</FormLabel>
                  <FormControl>
                    <Input
                      type="password"
                      placeholder="Your current password"
                      v-bind="componentField"
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              </FormField>

              <FormField name="newPassword" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>New Password</FormLabel>
                  <FormControl>
                    <Input
                      type="password"
                      placeholder="Your new password"
                      v-bind="componentField"
                    />
                  </FormControl>
                  <FormDescription>
                    Password must be at least 8 characters and include uppercase, lowercase, number, and special character.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              </FormField>

              <FormField name="confirmPassword" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>Confirm New Password</FormLabel>
                  <FormControl>
                    <Input
                      type="password"
                      placeholder="Confirm your new password"
                      v-bind="componentField"
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              </FormField>

              <Button type="submit" :disabled="loading.password">
                {{ loading.password ? "Changing Password..." : "Change Password" }}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>

      <!-- Account Information (Read-only) -->
      <Card>
        <CardHeader>
          <CardTitle>Account Information</CardTitle>
          <CardDescription>
            Your account details (read-only)
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div class="space-y-4">
            <div>
              <h3 class="text-sm font-medium">Username</h3>
              <p class="text-sm text-muted-foreground">{{ user?.username }}</p>
            </div>
            <div>
              <h3 class="text-sm font-medium">Account Created</h3>
              <p class="text-sm text-muted-foreground">
                {{ user?.created_at ? new Date(user.created_at).toLocaleString() : 'N/A' }}
              </p>
            </div>
            <div v-if="user?.super_admin">
              <h3 class="text-sm font-medium">Account Type</h3>
              <p class="text-sm text-muted-foreground pt-2">
                <span class="inline-flex items-center rounded-full bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700 ring-1 ring-inset ring-blue-700/10">
                  Super Admin
                </span>
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  </div>
</template>

<style scoped>
/* Add any page-specific styles here */
</style>
