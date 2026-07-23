import { createFileRoute, redirect } from "@tanstack/react-router";

import { NonAuthLayout } from "@/layouts/non-auth-layout";
import { $api } from "@/utils/api";
import { sessionOptions } from "@/utils/query-options";

export const Route = createFileRoute("/_auth")({
  beforeLoad: async ({ context: { queryClient } }) => {
    const session = await queryClient.ensureQueryData(sessionOptions);
    if (session) {
      redirect({
        params: { view: "my-drive" },
        search: {
          path: "/",
        },
        throw: true,
        to: "/$view",
      });
    }
  },
  component: NonAuthLayout,
});
