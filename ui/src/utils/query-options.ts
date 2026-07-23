import type { FileListParams, ShareListParams } from "@/types";
import type { components } from "@/lib/api";
import { infiniteQueryOptions, queryOptions, useQuery } from "@tanstack/react-query";
import type { FileData } from "file-browser";

import { getExtension, mediaUrl } from "./common";
import { defaultSortState, sortIdsMap, sortViewMap } from "./defaults";
import { NetworkError } from "./fetch-throw";
import { getPreviewType, preview } from "./preview-type";
import { fetchClient } from "./api";
import { useSettingsStore } from "./stores/settings";

export const sessionOptions = queryOptions({
  queryFn: async ({ signal }) => {
    try {
      const res = await fetchClient.GET("/auth/session", {
        signal,
      });
      if (res.response.status === 204) {
        return null;
      }
      return res.data;
    } catch (error) {
      if (error instanceof NetworkError && error.status === 401) {
        return null;
      }
      throw error;
    }
  },
  queryKey: ["session"],
});

export const useSession = () => {
  const { data, isLoading, refetch } = useQuery(sessionOptions);
  const status = isLoading ? "loading" : (data?.userId ? "success" : "unauthenticated");
  return [data ?? null, status, refetch] as const;
};

export const fileQueries = {
  list: (params: FileListParams, sessionHash?: string) =>
    infiniteQueryOptions({
      getNextPageParam: (lastPage) => lastPage?.meta.nextCursor || undefined,
      initialPageParam: undefined as string | undefined,
      queryFn: fetchFiles(params),
      queryKey: ["Files_list", params.view, params.params],
      select: (data) =>
        data.pages.flatMap((page) =>
          page?.items ? mapFilesToFb(page?.items!, sessionHash as string) : [],
        ),
    }),
};

export const shareQueries = {
  list: (params: ShareListParams) =>
    infiniteQueryOptions({
      getNextPageParam: (lastPage) => (lastPage as components["schemas"]["FileList"] | undefined)?.meta.nextCursor || undefined,
      initialPageParam: undefined as string | undefined,
      queryFn: async ({ pageParam, signal }) =>
        (
          await fetchClient.GET("/shares/{id}/files", {
            params: {
              path: { id: params.id },
              query: {
                ...(params.path ? { path: params.path } : {}),
                ...(pageParam ? { cursor: pageParam as string } : {}),
              },
            },
            signal,
          })
        ).data as components["schemas"]["FileList"] | undefined,
      queryKey: ["Shares_listFiles", params],
      select: (data) =>
        data.pages.flatMap((page) => {
          const fileList = page as components["schemas"]["FileList"] | undefined;
          return fileList?.items ? mapFilesToFb(fileList.items, "") : [];
        }),
    }),
};

const fetchFiles =
  (qparams: FileListParams) =>
  async ({ pageParam, signal }: { pageParam: string | undefined; signal: AbortSignal }) => {
    const { view, params } = qparams;
    const query: FileListParams["params"] = {
      order: view === "my-drive" ? defaultSortState.order : sortViewMap[view].order,
      sort:
        view === "my-drive"
          ? sortIdsMap[defaultSortState.sortId]
          : sortIdsMap[sortViewMap[view].sortId],
      ...(pageParam ? { cursor: pageParam } : {}),
    };
    if (view === "my-drive") {
      query.path = params?.path ?? "/";
    } else if (view === "search") {
      query.operation = "find";
      query.searchType = params?.searchType;
      for (const key in params) {
        if (key !== "path") {
          query[key] = params[key];
        }
      }
    } else if (view === "recent") {
      query.operation = "find";
      query.type = "file";
    } else if (view === "browse") {
      query.parentId = params?.parentId;
      if (params?.category) {
        query.operation = "find";
        query.type = "file";
        query.category = params?.category;
      }
    } else if (view === "shared") {
      query.operation = "find";
      query.shared = true;
    }

    return (
      await fetchClient.GET("/files", {
        params: {
          query,
        },
        signal,
      })
    ).data;
  };

const mapFilesToFb = (files: components["schemas"]["FileList"]["items"], sessionHash: string) => files.map((item): FileData => {
    if (item.mimeType === "drive/folder") {
      return {
        id: item.id!,
        isDir: true,
        mimeType: item.mimeType,
        modDate: item.updatedAt,
        name: item.name,
        size: item.size ? Number(item.size) : 0,
        type: item.type,
      };
    }

    const previewType = getPreviewType(getExtension(item.name), {
      video: item.mimeType?.includes("video"),
    });

    const { settings } = useSettingsStore.getState();

    let thumbnailUrl = "";
    if (previewType === "image") {
      if (settings.resizerHost) {
        const url = mediaUrl(item.id!, item.name, "", sessionHash);
        thumbnailUrl = settings.resizerHost
          ? `${settings.resizerHost}/insecure/w:360/plain/${encodeURIComponent(url)}`
          : "";
      }
    }
    return {
      id: item.id!,
      isEncrypted: item.encrypted,
      mimeType: item.mimeType,
      modDate: item.updatedAt,
      name: item.name,
      openable: !!preview[previewType!],
      previewType,
      size: item.size ? Number(item.size) : 0,
      thumbnailUrl,
      type: item.type,
    };
  });
