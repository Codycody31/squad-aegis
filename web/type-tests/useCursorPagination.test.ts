import assert from "node:assert/strict";
import test from "node:test";
// @ts-expect-error Node's strip-types runner requires the explicit extension.
import { useCursorPagination } from "../composables/useCursorPagination.ts";

test("moves forward and backward through opaque cursor history", () => {
  const pagination = useCursorPagination(50);

  assert.equal(pagination.page.value, 1);
  assert.equal(pagination.cursor.value, null);
  assert.equal(pagination.canGoPrevious.value, false);
  assert.equal(pagination.canGoNext.value, false);

  pagination.setPageInfo({
    next_cursor: "opaque-page-2",
    has_more: true,
  });
  assert.equal(pagination.next(), true);
  assert.equal(pagination.page.value, 2);
  assert.equal(pagination.cursor.value, "opaque-page-2");

  pagination.setPageInfo({
    next_cursor: "opaque-page-3",
    has_more: true,
  });
  assert.equal(pagination.next(), true);
  assert.equal(pagination.page.value, 3);
  assert.equal(pagination.cursor.value, "opaque-page-3");

  assert.equal(pagination.previous(), true);
  assert.equal(pagination.page.value, 2);
  assert.equal(pagination.cursor.value, "opaque-page-2");
  assert.equal(pagination.previous(), true);
  assert.equal(pagination.page.value, 1);
  assert.equal(pagination.cursor.value, null);
});

test("reset clears cursor history after a filter or page-size change", () => {
  const pagination = useCursorPagination(30);

  pagination.setPageInfo({
    next_cursor: "opaque-page-2",
    has_more: true,
  });
  assert.equal(pagination.next(), true);

  pagination.limit.value = 100;
  pagination.reset();

  assert.equal(pagination.limit.value, 100);
  assert.equal(pagination.page.value, 1);
  assert.equal(pagination.cursor.value, null);
  assert.equal(pagination.canGoPrevious.value, false);
  assert.equal(pagination.canGoNext.value, false);
});

test("does not advance when page metadata has no usable cursor", () => {
  const pagination = useCursorPagination(50);

  pagination.setPageInfo({ next_cursor: null, has_more: true });
  assert.equal(pagination.canGoNext.value, false);
  assert.equal(pagination.next(), false);
  assert.equal(pagination.page.value, 1);

  pagination.setPageInfo({
    next_cursor: "   ",
    has_more: true,
  });
  assert.equal(pagination.canGoNext.value, false);
  assert.equal(pagination.next(), false);
  assert.equal(pagination.previous(), false);
});
