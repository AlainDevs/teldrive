import { createFileRoute, redirect } from "@tanstack/react-router"

import { Settings } from "@/components/settings/settings"

export const Route = createFileRoute("/_authed/settings")({
  beforeLoad: ({ location }) => {
    if (location.pathname !== "/settings" && location.pathname !== "/settings/") {return}

    redirect({
      params: { tabId: "general" },
      throw: true,
      to: "/settings/$tabId",
    })
  },
  component: Settings,
})
