<script setup lang="ts">
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { Server } from "~/types";

useHead({
  title: "Dashboard",
});

definePageMeta({
  middleware: "auth",
});

const runtimeConfig = useRuntimeConfig();
const servers = ref<Server[]>([]);

const fetchServers = async () => {
  const { data, error } = await useFetch(
    `${runtimeConfig.public.backendApi}/servers`,
    {
      headers: {
        Authorization: `Bearer ${
          useCookie(runtimeConfig.public.sessionCookieName).value
        }`,
      },
    }
  );

  if (error.value) {
    console.error("Error fetching servers:", error.value);
  } else {
    servers.value = data.value?.data?.servers ?? [];
  }
};

await fetchServers();
</script>

<template>
  <div class="p-4">
    <h1 class="text-2xl font-bold">Dashboard</h1>
  </div>

  <div class="p-4">
    <Card>
      <CardHeader>
        <h2>Servers</h2>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Server Name</TableHead>
              <TableHead>IP Address</TableHead>
              <TableHead>Game Port</TableHead>
              <TableHead>RCON Port</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="server in servers" :key="server.id">
              <TableCell>{{ server.name }}</TableCell>
              <TableCell>{{ server.ip_address }}</TableCell>
              <TableCell>{{ server.game_port }}</TableCell>
              <TableCell>{{ server.rcon_port }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  </div>
</template>
