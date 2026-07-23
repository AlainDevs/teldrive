import { memo, useCallback, useEffect } from "react";
import { scrollbarClasses } from "@/utils/classes";
import clsx from "clsx";
import { Button } from "@heroui/react";
import { useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";

import { categoryConfig, generalSettingsConfig } from "@/config/settings";
import { SettingsField } from "./settings-field";
import { useSettingsStore } from "@/utils/stores/settings";
import { $api } from "@/utils/api";
import type { SettingValue } from "@/config/settings";

import IcBaselineCloudUpload from "~icons/ic/baseline-cloud-upload";
import IcBaselineSettings from "~icons/ic/baseline-settings";
import IcBaselineTv from "~icons/ic/baseline-tv";
import IcBaselineRestore from "~icons/ic/baseline-restore";

const iconMap: Record<string, React.ElementType> = {
  display: IcBaselineTv,
  other: IcBaselineSettings,
  upload: IcBaselineCloudUpload,
};

export const GeneralTab = memo(() => {
  const { settings, updateSetting, resetSettings } = useSettingsStore();

  const queryClient = useQueryClient();

  const { data: userConfig } = $api.useSuspenseQuery("get", "/users/config");

  const updateUserConfig = $api.useMutation("patch", "/users/config", {
    onError: () => {
      toast.error("Failed to update encryption setting");
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: $api.queryOptions("get", "/users/config").queryKey });
    },
  });

  useEffect(() => {
    updateSetting("encryptFiles", userConfig.encryptFiles);
  }, [updateSetting, userConfig.encryptFiles]);

  const categories = ["upload", "display", "other"] as const;

  const handleFieldChange = useCallback(
    async (key: keyof typeof settings, value: SettingValue) => {
      if (key === "encryptFiles") {
        await updateUserConfig.mutateAsync({ body: { encryptFiles: Boolean(value) } });
      }
      updateSetting(key, value);
    },
    [updateSetting, updateUserConfig],
  );

  const handleResetSettings = useCallback(async () => {
    try {
      await updateUserConfig.mutateAsync({ body: { encryptFiles: false } });
      resetSettings();
    } catch (error) {
      console.error("Failed to reset encryption setting", error);
    }
  }, [resetSettings, updateUserConfig]);

  return (
    <div className={clsx("flex flex-col gap-6 p-4 h-full overflow-y-auto", scrollbarClasses)}>
      {categories.map((category) => {
        const fields = generalSettingsConfig.filter((f) => f.category === category);
        if (fields.length === 0) {return null;}

        const catConfig = categoryConfig[category];
        const Icon = iconMap[category] || IcBaselineSettings;

        return (
          <div
            key={category}
            className="rounded-3xl p-6 bg-surface border border-border flex flex-col gap-6"
          >
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-2xl bg-accent-soft">
                <Icon className="size-6 text-accent-soft-foreground" />
              </div>
              <div className="flex-1 min-w-0">
                <h3 className="text-xl font-semibold mb-1">{catConfig.title}</h3>
                <p className="text-sm text-muted">{catConfig.description}</p>
              </div>
            </div>
            <div className="space-y-6">
              {fields.map((field) => (
                <SettingsField
                  key={field.key}
                  config={field}
                  value={settings[field.key] ?? field.defaultValue ?? ""}
                  onChange={(value) => handleFieldChange(field.key, value)}
                />
              ))}
            </div>
          </div>
        );
      })}
      <div className="mt-2 mb-6 flex justify-center">
        <Button
          variant="secondary"
          className="px-8 py-6 rounded-2xl font-semibold"
          isDisabled={updateUserConfig.isPending}
          onPress={handleResetSettings}
        >
          Reset All Settings
        </Button>
      </div>
    </div>
  );
});
