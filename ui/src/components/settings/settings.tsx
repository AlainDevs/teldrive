import { memo } from "react";
import { Outlet, getRouteApi, useNavigate } from "@tanstack/react-router";
import { Tabs } from "@heroui/react";
import CodiconAccount from "~icons/codicon/account";
import CodiconSettings from "~icons/codicon/settings";
import FluentDarkTheme20Filled from "~icons/fluent/dark-theme-20-filled";
import IcOutlineInfo from "~icons/ic/outline-info";
import MaterialSymbolsScheduleRounded from "~icons/material-symbols/schedule-rounded";

const tabItems = [
  { icon: CodiconSettings, id: "general" },
  { icon: FluentDarkTheme20Filled, id: "appearance" },
  { icon: CodiconAccount, id: "account" },
  { icon: MaterialSymbolsScheduleRounded, id: "jobs" },
  { icon: IcOutlineInfo, id: "info" },
];

const fileRoute = getRouteApi("/_authed/settings/$tabId");

export const Settings = memo(() => {
  const params = fileRoute.useParams();
  const navigate = useNavigate();

  return (
    <div className="bg-surface/50 mx-auto size-full rounded-xl flex flex-col md:flex-row max-w-5xl gap-6 p-4">
      <div className="flex flex-col gap-4 w-full md:w-48 shrink-0">
        <h1 className="text-2xl font-semibold pt-2 px-2">Settings</h1>
        <Tabs
          selectedKey={params.tabId}
          onSelectionChange={(key) =>
            navigate({ to: "/settings/$tabId", params: { tabId: key.toString() }, replace: true })
          }
          orientation="horizontal"
          variant="secondary"
        >
          <Tabs.ListContainer>
            <Tabs.List
              aria-label="Settings tabs"
              className="max-md:!flex-row md:flex-col gap-1 flex-nowrap overflow-x-auto md:overflow-visible no-scrollbar"
            >
              {tabItems.map((tab) => {
                const Icon = tab.icon;
                return (
                  <Tabs.Tab
                    key={tab.id}
                    id={tab.id}
                    className="h-12 justify-center md:justify-start px-4 gap-3 data-[selected=true]:text-accent shrink-0 w-auto md:w-full"
                  >
                    <Icon className="size-5" />
                    <span className="capitalize text-xs md:text-sm hidden md:inline">{tab.id}</span>
                    <Tabs.Indicator className="bg-accent-soft rounded-full" />
                  </Tabs.Tab>
                );
              })}
            </Tabs.List>
          </Tabs.ListContainer>
          {tabItems.map((tab) => (
            <Tabs.Panel key={tab.id} id={tab.id} className="hidden" />
          ))}
        </Tabs>
      </div>
      <div className="flex-1 overflow-hidden">
        <Outlet />
      </div>
    </div>
  );
});
