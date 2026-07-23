export enum SortOrder {
  ASC = "asc",
  DESC = "desc",
}

export const sortViewMap = {
  browse: {
    order: SortOrder.ASC,
    sortId: "sort_files_by_name",
  },
  category: { order: SortOrder.ASC, sortId: "sort_files_by_name" },
  "my-drive": {
    order: SortOrder.ASC,
    sortId: "sort_files_by_name",
  },
  recent: { order: SortOrder.DESC, sortId: "sort_files_by_date" },
  search: { order: SortOrder.ASC, sortId: "sort_files_by_name" },
  shared: { order: SortOrder.DESC, sortId: "sort_files_by_date" },
};

export type SortState = typeof sortViewMap;

export const getSortState = () =>
  (JSON.parse(localStorage.getItem("sort")!) as null | SortState["my-drive"]) ||
  sortViewMap["my-drive"];

export const defaultSortState = getSortState();

export const defaultViewId = localStorage.getItem("viewId") || "enable_list_view";

export const sortIdsMap = {
  sort_files_by_date: "updatedAt",
  sort_files_by_name: "name",
  sort_files_by_size: "size",
} as const;

export const BREAKPOINTS = { lg: 992, md: 576, sm: 476, xs: 0 };
