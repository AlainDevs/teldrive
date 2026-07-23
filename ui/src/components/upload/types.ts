import type { FileUploadStatus } from "@/utils/stores";

export interface CircularProgressProps {
  progress: number;
  size?: number;
  strokeWidth?: number;
  className?: string;
  showCancel?: boolean;
  onCancel?: () => void;
  status?: FileUploadStatus;
}

export type UploadParams = Record<
  string,
  string | number | boolean | undefined
>;

export interface UploadProps {
  queryKey: any[];
  mode?: "drive" | "share";
  shareId?: string;
  path?: string;
  userId?: number;
  encryptFiles?: boolean;
}

export interface UploadFileOptions {
  shareId?: string;
}
