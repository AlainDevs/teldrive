import { createFileRoute } from "@tanstack/react-router";

const redirectBase = "http://teldrive.local";

const validateRedirect = (redirect: unknown) => {
  if (typeof redirect !== "string") {
    return undefined;
  }

  try {
    const url = new URL(redirect, redirectBase);
    if (url.origin !== redirectBase) {
      return undefined;
    }
    return `${url.pathname}${url.search}${url.hash}`;
  } catch {
    return undefined;
  }
};

export const Route = createFileRoute("/_auth/login")({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: validateRedirect(search.redirect),
  }),
});
