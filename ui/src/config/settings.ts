export type FieldType =
  | "text"
  | "number"
  | "email"
  | "url"
  | "select"
  | "switch"
  | "textarea";

type SettingKeys =
  | "uploadConcurrency"
  | "uploadRetries"
  | "uploadRetryDelay"
  | "randomChunking"
  | "resizerHost"
  | "pageSize"
  | "splitFileSize"
  | "encryptFiles"
  | "rcloneProxy"
  | "taskPollingInterval";

type SettingValue = string | number | boolean;

export interface SettingFieldConfig<T> {
  key: SettingKeys;
  type: FieldType;
  label: string;
  description: string;
  placeholder?: string;
  defaultValue?: T;
  options?: { value: T; label: string }[];
  validation?: {
    pattern?: RegExp;
    custom?: (value: SettingValue) => string | true;
  };
  category: "upload" | "display" | "security" | "other";
}

const splitFileSizes = [
  { label: "100MB", value: 100 * 1024 * 1024 },
  { label: "500MB", value: 500 * 1024 * 1024 },
  { label: "1GB", value: 1000 * 1024 * 1024 },
  { label: "2GB", value: 2 * 1000 * 1024 * 1024 },
];

export const generalSettingsConfig: SettingFieldConfig<SettingValue>[] = [
  {
    category: "upload",
    defaultValue: 4,
    description: "Concurrent Part Uploads",
    key: "uploadConcurrency",
    label: "Concurrency",
    type: "number",
  },
  {
    category: "upload",
    defaultValue: 3,
    description: "Number of retries for each part upload",
    key: "uploadRetries",
    label: "Upload Retries",
    type: "number",
  },
  {
    category: "upload",
    defaultValue: 1000,
    description: "Delay between retries in milliseconds",
    key: "uploadRetryDelay",
    label: "Upload Retry Delay",
    type: "number",
  },
  {
    category: "other",
    description: "Image Resize Host to resize images",
    key: "resizerHost",
    label: "Resizer Host",
    placeholder: "https://resizer.example.com",
    type: "url",
  },
  {
    category: "display",
    defaultValue: 500,
    description: "Number of items per page",
    key: "pageSize",
    label: "Page Size",
    type: "number",
  },
  {
    category: "upload",
    defaultValue: splitFileSizes[1].value,
    description: "Split File Size for multipart uploads",
    key: "splitFileSize",
    label: "Split File Size",
    options: splitFileSizes,
    type: "select",
  },
  {
    category: "upload",
    defaultValue: false,
    description: "Encrypt Files before uploading",
    key: "encryptFiles",
    label: "Encrypt Files",
    type: "switch",
  },
  {
    category: "upload",
    defaultValue: true,
    description: "Randomize Names of File Chunks",
    key: "randomChunking",
    label: "Random Chunking",
    type: "switch",
  },
  {
    category: "other",
    description: "Play Files directly from Rclone Webdav",
    key: "rcloneProxy",
    label: "Rclone Media Proxy",
    placeholder: "http://localhost:8080",
    type: "url",
  },
];

type LiteralToPrimitive<T> = T extends boolean
  ? boolean
  : T extends number
    ? number
    : T extends string
      ? string
      : T;

export type Settings = {
  [P in (typeof generalSettingsConfig)[number] as P["key"]]: LiteralToPrimitive<
    P["defaultValue"]
  >;
};

export function getSettingsValues(): Settings {
  const settings = {} as any;
  for (const item of generalSettingsConfig) {
    if (item.defaultValue !== undefined) {
      settings[item.key] = item.defaultValue;
    } else {
      switch (item.type) {
        case "number":
          settings[item.key] = 0;
          break;
        case "switch":
          settings[item.key] = false;
          break;
        default:
          settings[item.key] = "";
      }
    }
  }
  return settings;
}

export const categoryConfig = {
  display: {
    description: "Customize how content is displayed",
    title: "Display",
  },
  other: {
    description: "Other Options",
    title: "Other",
  },
  security: {
    description: "Configure security options",
    title: "Security",
  },
  upload: {
    description: "Configure upload behavior",
    title: "Uploads",
  },
} as const;
