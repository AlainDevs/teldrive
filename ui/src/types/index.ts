import type { operations } from "@/lib/api";
import type { Dispatch, SetStateAction } from "react";

export interface FileResponse {
  files: SingleFile[];
  meta: { totalPages: number; count: number; currentPage: number };
}

export interface SingleFile {
  name: string;
  type: string;
  mimeType: string;
  size: number;
  depth: number;
  createdAt: string;
  updatedAt: string;
  userId: string;
  parentId: string;
  id: string;
  encrypted?: boolean;
  path?: string;
}

export interface FilePayload {
  id?: string;
  payload?: Record<string, any>;
}

export interface UploadPart {
  name: string;
  partId: number;
  partNo: number;
  size: number;
  channelId: number;
  encrypted?: boolean;
  salt?: string;
}

export interface Message {
  message: string;
  error?: string;
  code?: number;
}

export interface Session {
  name: string;
  userName: string;
  userId: number;
  isPremium: boolean;
  sessionId: string;
  hash?: string;
  expires: string;
}

export interface UserSession {
  sessionId: string;
  hash?: string;
  createdAt: string;
  location?: string;
  officialApp?: boolean;
  appName?: string;
  valid: boolean;
  current: boolean;
}

export type BrowseView = "my-drive" | "search" | "recent" | "browse" | "shared";

export interface FileListParams {
  view: BrowseView;
  params: Exclude<operations["Files_list"]["parameters"]["query"], undefined>;
}

export interface ShareListParams {
  id: string;
  path?: string;
}

export interface AccountStats {
  channelId: number;
  bots: string[];
}

export interface Channel {
  channelName?: string;
  channelId: number;
}

export type Tags = Record<string, any>;

export interface AudioMetadata {
  artist: string;
  title: string;
  cover: string;
}

export interface UploadStats {
  uploadDate: string;
  totalUploaded: number;
}

export interface CategoryStorage {
  category: string;
  totalFiles: number;
  totalSize: number;
}

export interface FileShare {
  id: string;
  expirationDate: string;
  protected: boolean;
  type: string;
  name: string;
}

export type SetValue<T> = Dispatch<SetStateAction<T>>;

export interface PreviewFile {
  id: string;
  name: string;
  mimeType: string;
  previewType: string;
}
