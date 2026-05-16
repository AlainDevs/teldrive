import type { QueryClient } from "@tanstack/react-query";
import { type ParsedLocation, createFileRoute, redirect } from "@tanstack/react-router";

import { AuthLayout } from "@/layouts/auth-layout";
import { sessionOptions } from "@/utils/query-options";

const checkAuth = async (queryClient: QueryClient, location: ParsedLocation, preload: boolean) => {
  if (preload) {
    return;
  }
  const session = await queryClient.ensureQueryData(sessionOptions);
  if (!session) {
    redirect({
      search: {
        redirect: location.href,
      },
      throw: true,
      to: "/login",
    });
  }
};

export const Route = createFileRoute("/_authed")({
  beforeLoad: ({ location, context: { queryClient }, preload }) =>
    checkAuth(queryClient, location, preload),
  component: AuthLayout,
});
