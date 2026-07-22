<script setup lang="ts">
import { ref } from "vue";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "~/components/ui/select";
import { Checkbox } from "~/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "~/components/ui/dialog";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "~/components/ui/form";
import { toast } from "~/components/ui/toast";
import { toTypedSchema } from "@vee-validate/zod";
import * as z from "zod";

const emit = defineEmits<{
  (e: "created"): void;
}>();

const runtimeConfig = useRuntimeConfig();
const showDialog = ref(false);
const submitting = ref(false);

// Track selected log source type for conditional fields
const selectedLogSourceType = ref<string | null>(null);

// IPv4 / IPv6 literal validator. The backend stores ip_address as PostgreSQL
// `inet`, which rejects hostnames — surface the constraint up front so users
// see the right field flagged instead of "Bad Request".
const ipAddressSchema = z
  .string()
  .min(1, "IP address is required")
  .refine((v) => z.string().ip().safeParse(v.trim()).success, {
    message:
      "Must be a valid IPv4 or IPv6 address (hostnames are not supported)",
  });

const formSchema = toTypedSchema(
  z.object({
    name: z.string().min(1, "Server name is required"),
    ip_address: ipAddressSchema,
    game_port: z.coerce
      .number()
      .min(1, "Game port is required")
      .max(65535, "Port must be between 1 and 65535"),
    rcon_ip_address: z
      .string()
      .optional()
      .nullable()
      .refine((v) => !v || z.string().ip().safeParse(v.trim()).success, {
        message: "Must be a valid IPv4 or IPv6 address",
      }),
    rcon_port: z.coerce
      .number()
      .min(1, "RCON port is required")
      .max(65535, "Port must be between 1 and 65535"),
    rcon_password: z.string().min(1, "RCON password is required"),

    // Log & file access configuration fields
    log_source_type: z.enum(["local", "sftp", "ftp"], {
      required_error: "Log source type is required",
    }),
    squad_game_path: z.string().min(1, "SquadGame base path is required"),
    log_host: z.string().optional().nullable(),
    log_port: z
      .preprocess(
        (val) => (val === "" || val === null || val === undefined ? null : val),
        z.coerce.number().min(1).max(65535).nullable()
      )
      .optional(),
    log_username: z.string().optional().nullable(),
    log_password: z.string().optional().nullable(),
    log_poll_frequency: z
      .preprocess(
        (val) => (val === "" || val === null || val === undefined ? null : val),
        z.coerce.number().min(1).max(300).nullable()
      )
      .optional(),
    log_read_from_start: z.boolean().optional().nullable(),
  })
);

const initialValues = {
  name: "",
  ip_address: "",
  game_port: 7787,
  rcon_ip_address: null,
  rcon_port: 21114,
  rcon_password: "",
  log_source_type: undefined,
  squad_game_path: "",
  log_host: null,
  log_port: null,
  log_username: null,
  log_password: null,
  log_poll_frequency: 2,
  log_read_from_start: false,
};

async function addServer(values: any) {
  const {
    name,
    ip_address,
    game_port,
    rcon_ip_address,
    rcon_port,
    rcon_password,
    log_source_type,
    squad_game_path,
    log_host,
    log_port,
    log_username,
    log_password,
    log_poll_frequency,
    log_read_from_start,
  } = values;

  submitting.value = true;

  try {
    await useAuthFetchImperative(
      `${runtimeConfig.public.backendApi}/servers`,
      {
        method: "POST",
        body: {
          name,
          ip_address,
          game_port: parseInt(game_port),
          rcon_ip_address,
          rcon_port: parseInt(rcon_port),
          rcon_password,

          // Log configuration
          log_source_type: log_source_type || null,
          squad_game_path: squad_game_path || null,
          log_host: log_host || null,
          log_port: log_port ? parseInt(log_port) : null,
          log_username: log_username || null,
          log_password: log_password || null,
          log_poll_frequency: log_poll_frequency
            ? parseInt(log_poll_frequency)
            : null,
          log_read_from_start: log_read_from_start || false,
        },
      }
    );

    showDialog.value = false;
    selectedLogSourceType.value = null;
    toast({
      title: "Server added",
      description: `${name} is now managed by Squad Aegis.`,
    });
    emit("created");
  } catch (err: any) {
    toast({
      title: "Failed to add server",
      description: extractApiErrorMessage(
        err,
        "An error occurred while adding the server"
      ),
      variant: "destructive",
    });
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Form
    v-slot="{ handleSubmit }"
    as=""
    keep-values
    :validation-schema="formSchema"
    :initial-values="initialValues"
  >
    <Dialog v-model:open="showDialog">
      <DialogTrigger asChild>
        <Button class="w-full sm:w-auto">Add Server</Button>
      </DialogTrigger>
      <DialogContent
        class="max-h-[90vh] w-[95vw] overflow-y-auto p-4 sm:max-w-[425px] sm:p-6"
      >
        <DialogHeader>
          <DialogTitle class="text-base sm:text-lg">Add New Server</DialogTitle>
          <DialogDescription class="text-xs sm:text-sm">
            Enter the details of the Squad server you want to add.
          </DialogDescription>
        </DialogHeader>
        <form id="dialogForm" @submit="handleSubmit($event, addServer)">
          <div class="grid gap-4 py-4">
            <FormField name="name" v-slot="{ componentField }">
              <FormItem>
                <FormLabel>Server Name</FormLabel>
                <FormControl>
                  <Input placeholder="My Squad Server" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField name="ip_address" v-slot="{ componentField }">
              <FormItem>
                <FormLabel>IP Address</FormLabel>
                <FormControl>
                  <Input placeholder="127.0.0.1" v-bind="componentField" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField name="game_port" v-slot="{ componentField }">
              <FormItem>
                <FormLabel>Game Port</FormLabel>
                <FormControl>
                  <Input type="number" placeholder="7787" v-bind="componentField" />
                </FormControl>
                <FormDescription> Default Squad game port is 7787 </FormDescription>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField name="rcon_ip_address" v-slot="{ componentField }">
              <FormItem>
                <FormLabel>RCON IP Address</FormLabel>
                <FormControl>
                  <Input placeholder="e.g., 192.168.1.1" v-bind="componentField" />
                </FormControl>
                <FormDescription>
                  If your RCON IP address is different from the game IP address,
                  specify it here. Otherwise, leave it blank.
                </FormDescription>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField name="rcon_port" v-slot="{ componentField }">
              <FormItem>
                <FormLabel>RCON Port</FormLabel>
                <FormControl>
                  <Input type="number" placeholder="21114" v-bind="componentField" />
                </FormControl>
                <FormDescription> Default Squad RCON port is 21114 </FormDescription>
                <FormMessage />
              </FormItem>
            </FormField>
            <FormField name="rcon_password" v-slot="{ componentField }">
              <FormItem>
                <FormLabel>RCON Password</FormLabel>
                <FormControl>
                  <Input
                    type="password"
                    placeholder="********"
                    v-bind="componentField"
                  />
                </FormControl>
                <FormDescription>
                  The RCON password for your Squad server
                </FormDescription>
                <FormMessage />
              </FormItem>
            </FormField>

            <!-- Log & File Access Configuration Section -->
            <div class="mt-4 border-t pt-4">
              <h4 class="mb-3 text-sm font-medium">Log & File Access</h4>
              <p class="mb-4 text-xs text-muted-foreground">
                Configure file access for log monitoring, event tracking, and
                Bans.cfg management.
              </p>
              <p class="mb-4 text-xs text-muted-foreground">
                Log source and base path are required for ban enforcement and
                config sync.
              </p>

              <FormField name="log_source_type" v-slot="{ componentField }">
                <FormItem>
                  <FormLabel>Log Source Type</FormLabel>
                  <Select
                    v-model="selectedLogSourceType"
                    @update:modelValue="componentField.onChange"
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder="Select log source type" />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value="local">Local File</SelectItem>
                      <SelectItem value="sftp">SFTP</SelectItem>
                      <SelectItem value="ftp">FTP</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    "Local" if Aegis runs on the same machine as your Squad
                    server. "SFTP" or "FTP" for remote server access.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              </FormField>

              <FormField
                name="squad_game_path"
                v-slot="{ componentField }"
                v-if="selectedLogSourceType"
              >
                <FormItem>
                  <FormLabel>SquadGame Base Path</FormLabel>
                  <FormControl>
                    <Input
                      :placeholder="
                        selectedLogSourceType === 'local'
                          ? '/home/squad/serverfiles/SquadGame'
                          : '/SquadGame'
                      "
                      v-bind="componentField"
                    />
                  </FormControl>
                  <FormDescription>
                    Base path to the SquadGame folder. Aegis derives log and
                    config paths from this (Saved/Logs, ServerConfig).
                  </FormDescription>
                  <FormDescription v-if="selectedLogSourceType === 'local'">
                    When running in Docker, this folder must be mounted into the
                    container and readable by Aegis.
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              </FormField>

              <!-- Remote connection fields for SFTP/FTP -->
              <template
                v-if="
                  selectedLogSourceType === 'sftp' ||
                  selectedLogSourceType === 'ftp'
                "
              >
                <FormField name="log_host" v-slot="{ componentField }">
                  <FormItem>
                    <FormLabel>
                      {{ selectedLogSourceType?.toUpperCase() }} Host
                    </FormLabel>
                    <FormControl>
                      <Input placeholder="192.168.1.100" v-bind="componentField" />
                    </FormControl>
                    <FormDescription>
                      Hostname or IP address of the
                      {{ selectedLogSourceType?.toUpperCase() }} server
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField name="log_port" v-slot="{ componentField }">
                  <FormItem>
                    <FormLabel>
                      {{ selectedLogSourceType?.toUpperCase() }} Port
                    </FormLabel>
                    <FormControl>
                      <Input
                        type="number"
                        :placeholder="selectedLogSourceType === 'sftp' ? '22' : '21'"
                        v-bind="componentField"
                      />
                    </FormControl>
                    <FormDescription>
                      Port for {{ selectedLogSourceType?.toUpperCase() }}
                      connection
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField name="log_username" v-slot="{ componentField }">
                  <FormItem>
                    <FormLabel>Username</FormLabel>
                    <FormControl>
                      <Input placeholder="username" v-bind="componentField" />
                    </FormControl>
                    <FormDescription>
                      Username for {{ selectedLogSourceType?.toUpperCase() }}
                      authentication
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField name="log_password" v-slot="{ componentField }">
                  <FormItem>
                    <FormLabel>Password</FormLabel>
                    <FormControl>
                      <Input
                        type="password"
                        placeholder="********"
                        v-bind="componentField"
                      />
                    </FormControl>
                    <FormDescription>
                      Password for {{ selectedLogSourceType?.toUpperCase() }}
                      authentication
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                </FormField>

                <FormField name="log_poll_frequency" v-slot="{ componentField }">
                  <FormItem>
                    <FormLabel>Poll Frequency (seconds)</FormLabel>
                    <FormControl>
                      <Input type="number" placeholder="2" v-bind="componentField" />
                    </FormControl>
                    <FormDescription>
                      How often to check for new log entries (1-300 seconds).
                      2-4 seconds recommended for fast enforcement; higher
                      values can delay kicks.
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                </FormField>
              </template>

              <FormField
                name="log_read_from_start"
                v-slot="{ componentField }"
                v-if="selectedLogSourceType"
              >
                <FormItem class="flex flex-row items-start space-x-3 space-y-0">
                  <FormControl>
                    <Checkbox
                      :checked="componentField.modelValue"
                      @update:checked="componentField.onChange"
                    />
                  </FormControl>
                  <div class="space-y-1 leading-none">
                    <FormLabel> Read from start of file </FormLabel>
                    <FormDescription>
                      Process the entire log file from the beginning instead of
                      just new entries
                    </FormDescription>
                  </div>
                  <FormMessage />
                </FormItem>
              </FormField>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" @click="showDialog = false">
              Cancel
            </Button>
            <Button type="submit" :disabled="submitting">
              {{ submitting ? "Adding..." : "Add Server" }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </Form>
</template>
