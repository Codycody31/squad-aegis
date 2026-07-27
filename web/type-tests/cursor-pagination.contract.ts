import type { CursorPaginationController } from "../composables/useCursorPagination";
import type { PageInfo } from "../types";

declare const pagination: CursorPaginationController;

// Page size is the one intentionally writable part of the controller.
pagination.limit.value = 100;

// Callers can drive the state machine only through its public transitions.
pagination.setPageInfo({ next_cursor: "opaque-cursor", has_more: true });
pagination.next();
pagination.previous();
pagination.reset();

const currentPage: number = pagination.page.value;
const canAdvance: boolean = pagination.canGoNext.value;
const canReturn: boolean = pagination.canGoPrevious.value;
void currentPage;
void canAdvance;
void canReturn;

// Opaque cursors and the history stack cannot be mutated by callers.
// @ts-expect-error cursor changes must go through next(), previous(), or reset()
pagination.cursor.value = "caller-supplied-cursor";
// @ts-expect-error the cursor history is deliberately private
pagination.cursorStack;

const terminalPage = {
  next_cursor: null,
  has_more: false,
} satisfies PageInfo;
void terminalPage;

// Totals and numeric page metadata are intentionally absent from list results.
// @ts-expect-error legacy total metadata is not part of PageInfo
const legacyPage: PageInfo = { next_cursor: null, has_more: false, total: 10 };
void legacyPage;

export {};
