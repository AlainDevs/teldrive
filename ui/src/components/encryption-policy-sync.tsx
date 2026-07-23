import { useEffect, useRef } from "react";

import { $api } from "@/utils/api";
import { useSession } from "@/utils/query-options";
import { useSettingsStore } from "@/utils/stores/settings";

export const EncryptionPolicySync = () => {
  const [session] = useSession();
  const updateSetting = useSettingsStore((state) => state.updateSetting);
  const syncStarted = useRef(false);
  const { data: userConfig } = $api.useQuery("get", "/users/config", {
    enabled: Boolean(session?.userId),
  });
  const updateUserConfig = $api.useMutation("patch", "/users/config");

  useEffect(() => {
    if (!session?.userId || !userConfig || syncStarted.current) {
      return;
    }
    syncStarted.current = true;

    const migrationKey = `encrypt-files-server-policy:${session.userId}`;
    const migrated = localStorage.getItem(migrationKey) === "true";
    const localEncryptFiles = Boolean(useSettingsStore.getState().settings.encryptFiles);
    const encryptFiles = migrated
      ? userConfig.encryptFiles
      : userConfig.encryptFiles || localEncryptFiles;

    updateSetting("encryptFiles", encryptFiles);
    if (migrated || encryptFiles === userConfig.encryptFiles) {
      localStorage.setItem(migrationKey, "true");
      return;
    }

    updateUserConfig.mutate(
      { body: { encryptFiles } },
      {
        onError: () => updateSetting("encryptFiles", userConfig.encryptFiles),
        onSuccess: () => localStorage.setItem(migrationKey, "true"),
      },
    );
  }, [session?.userId, updateSetting, updateUserConfig, userConfig]);

  return null;
};
