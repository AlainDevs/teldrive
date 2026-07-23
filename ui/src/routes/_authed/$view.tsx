import type { BrowseView, FileListParams } from "@/types";
import { createFileRoute } from "@tanstack/react-router";

import { fileQueries } from "@/utils/query-options";
import { ErrorView } from "@/components/error-view";

const allowedTypes = new Set(["my-drive", "recent", "search", "storage", "browse", "shared"]);

export const Route = createFileRoute("/_authed/$view")({
  beforeLoad: ({ params }) => {
    if (!allowedTypes.has(params.view)) {
      throw new Error("invalid path");
    }
  },
  errorComponent: ({ error }) => <ErrorView message={error.message} />,
  loader: async ({ context: { queryClient }, deps, params }) => {
    await queryClient.ensureInfiniteQueryData(
      fileQueries.list({ params: deps, view: params.view as BrowseView }),
    );
  },
  loaderDeps: ({ search }) => search,
  validateSearch: (search: Record<string, unknown>) => (search || {}) as FileListParams["params"],
  wrapInSuspense: true,
});
