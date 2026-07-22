export interface DataTableColumn<T = any> {
  /** Column id; also names the cell slot (#cell-<key>) */
  key: string;
  label: string;
  /** Classes applied to both th and td */
  class?: string;
  sortable?: boolean;
  /** Omit this column from the mobile card view */
  hideOnMobile?: boolean;
  /** Label shown in the mobile card (defaults to label) */
  mobileLabel?: string;
  /** Cell value when no #cell-<key> slot is provided; also used for client sorting */
  accessor?: (row: T) => unknown;
}

export interface DataTableSort {
  key: string;
  dir: "asc" | "desc";
}
