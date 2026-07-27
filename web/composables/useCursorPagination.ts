import { computed, ref } from "vue";
import type { ComputedRef, Ref } from "vue";
import type { PageInfo } from "~/types";

const emptyPageInfo = (): PageInfo => ({
  next_cursor: null,
  has_more: false,
});

export interface CursorPaginationController {
  /**
   * The page size remains caller-controlled. Changing it must be followed by
   * reset() before the next request.
   */
  readonly limit: Ref<number>;
  /** The opaque cursor to send with the next request. */
  readonly cursor: Readonly<Ref<string | null>>;
  /** A display-only page number derived from the private cursor stack. */
  readonly page: ComputedRef<number>;
  readonly canGoPrevious: ComputedRef<boolean>;
  readonly canGoNext: ComputedRef<boolean>;
  readonly setPageInfo: (
    value?: Readonly<Partial<PageInfo>> | null,
  ) => void;
  readonly reset: () => void;
  readonly next: () => boolean;
  readonly previous: () => boolean;
}

/**
 * State shared by server-backed keyset lists.
 *
 * The stack stores the cursor used to fetch each previous page. Cursors remain
 * opaque to the client; filters and page size must call reset() before fetching.
 */
export function useCursorPagination(
  initialLimit: number,
): CursorPaginationController {
  const limit = ref(initialLimit);
  const cursor = ref<string | null>(null);
  const cursorStack = ref<Array<string | null>>([]);
  const pageInfo = ref<PageInfo>(emptyPageInfo());

  const page = computed(() => cursorStack.value.length + 1);
  const canGoPrevious = computed(() => cursorStack.value.length > 0);
  const canGoNext = computed(
    () => pageInfo.value.has_more && Boolean(pageInfo.value.next_cursor),
  );

  function setPageInfo(value?: Readonly<Partial<PageInfo>> | null) {
    const nextCursor = value?.next_cursor?.trim() || null;
    pageInfo.value = {
      next_cursor: nextCursor,
      // Advancing without a cursor is impossible. Treat inconsistent API
      // payloads as a terminal page instead of exposing a broken Next button.
      has_more: Boolean(value?.has_more && nextCursor),
    };
  }

  function reset() {
    cursor.value = null;
    cursorStack.value = [];
    pageInfo.value = emptyPageInfo();
  }

  function next(): boolean {
    if (!canGoNext.value || !pageInfo.value.next_cursor) {
      return false;
    }

    cursorStack.value.push(cursor.value);
    cursor.value = pageInfo.value.next_cursor;
    pageInfo.value = emptyPageInfo();
    return true;
  }

  function previous(): boolean {
    if (!canGoPrevious.value) {
      return false;
    }

    cursor.value = cursorStack.value.pop() ?? null;
    pageInfo.value = emptyPageInfo();
    return true;
  }

  return {
    limit,
    cursor,
    page,
    canGoPrevious,
    canGoNext,
    setPageInfo,
    reset,
    next,
    previous,
  };
}
