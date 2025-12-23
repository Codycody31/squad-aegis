<script setup lang="ts">
import { ref, onMounted } from "vue";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "~/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "~/components/ui/table";
import { Button } from "~/components/ui/button";
import type { PostgreSQLStats, ClickHouseStats } from "~/types";

definePageMeta({ middleware: "auth", layout: "sudo" });

const runtimeConfig = useRuntimeConfig();
const authStore = useAuthStore();

if (!authStore.user?.super_admin) navigateTo("/dashboard");

const loading = ref(true);
const pgStats = ref<PostgreSQLStats | null>(null);
const chStats = ref<ClickHouseStats | null>(null);

const fetchData = async () => {
  loading.value = true;
  try {
    const [pgRes, chRes] = await Promise.all([
      $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/database/postgresql`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      }),
      $fetch<any>(`${runtimeConfig.public.backendApi}/sudo/database/clickhouse`, {
        headers: { Authorization: `Bearer ${authStore.token}` },
      }),
    ]);
    pgStats.value = pgRes.data.data;
    chStats.value = chRes.data.data;
  } catch (err: any) {
    console.error("Error fetching database stats:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(fetchData);
</script>

<template>
  <div class="p-6 space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold">Database Statistics</h1>
        <p class="text-muted-foreground">View database health and statistics</p>
      </div>
      <Button @click="fetchData"><Icon name="mdi:refresh" class="mr-2" />Refresh</Button>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="text-muted-foreground">Loading database statistics...</div>
    </div>

    <Tabs v-else default-value="postgresql" class="space-y-4">
      <TabsList>
        <TabsTrigger value="postgresql">PostgreSQL</TabsTrigger>
        <TabsTrigger value="clickhouse">ClickHouse</TabsTrigger>
      </TabsList>

      <TabsContent value="postgresql" class="space-y-4">
        <div class="grid gap-4 md:grid-cols-4">
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm font-medium">Database Size</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ pgStats?.database_size }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm font-medium">Tables</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ pgStats?.table_count }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm font-medium">Connections</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ pgStats?.total_connections }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm font-medium">Cache Hit Ratio</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ pgStats?.cache_hit_ratio.toFixed(1) }}%</div>
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Tables</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Table</TableHead>
                  <TableHead>Rows</TableHead>
                  <TableHead>Size</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="table in pgStats?.tables" :key="table.table_name">
                  <TableCell>{{ table.table_name }}</TableCell>
                  <TableCell>{{ table.row_count.toLocaleString() }}</TableCell>
                  <TableCell>{{ table.total_size }}</TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="clickhouse" class="space-y-4">
        <div class="grid gap-4 md:grid-cols-3">
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm font-medium">Total Rows</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ chStats?.total_rows.toLocaleString() }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm font-medium">Total Size</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ chStats?.total_bytes }}</div>
            </CardContent>
          </Card>
          <Card>
            <CardHeader class="pb-2">
              <CardTitle class="text-sm font-medium">Tables</CardTitle>
            </CardHeader>
            <CardContent>
              <div class="text-2xl font-bold">{{ chStats?.table_count }}</div>
            </CardContent>
          </Card>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Tables</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Table</TableHead>
                  <TableHead>Rows</TableHead>
                  <TableHead>Size</TableHead>
                  <TableHead>Compression</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="table in chStats?.tables" :key="table.table_name">
                  <TableCell>{{ table.table_name }}</TableCell>
                  <TableCell>{{ table.total_rows.toLocaleString() }}</TableCell>
                  <TableCell>{{ table.total_bytes }}</TableCell>
                  <TableCell>{{ table.compression_ratio }}x</TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </TabsContent>
    </Tabs>
  </div>
</template>

