import { computed } from 'vue';
import { useRoute } from 'vue-router';

export function useBreadcrumbs() {
  const route = useRoute();

  // Compute breadcrumbs
  const breadcrumbs = computed(() => {
    const pathname = route.path;

    // Fallback: Generate breadcrumbs dynamically from the path
    const segments = pathname.split('/').filter(Boolean);
    return segments.map((segment, index) => {
      const path = `/${segments.slice(0, index + 1).join('/')}`;
      return {
        title: segment.charAt(0).toUpperCase() + segment.slice(1),
        link: path
      };
    });
  });

  return breadcrumbs;
}
