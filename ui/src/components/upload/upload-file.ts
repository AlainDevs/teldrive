import md5 from "md5";
import pLimit from "p-limit";


import type { components } from "@/lib/api";
import { fetchClient } from "@/utils/api";
import { formatTime, zeroPad } from "@/utils/common";
import type { UploadFileOptions, UploadParams } from "./types";


function generateUUID(): string {
  if (window.crypto && typeof window.crypto.randomUUID === "function") {
    return window.crypto.randomUUID();
  }

  if (window.crypto && typeof window.crypto.getRandomValues === "function") {
    const randomValues = window.crypto.getRandomValues(new Uint8Array(16));
    randomValues[6] = (randomValues[6] & 0x0f) | 0x40;
    randomValues[8] = (randomValues[8] & 0x3f) | 0x80;

    const hex = Array.from(randomValues, (value) =>
      value.toString(16).padStart(2, "0"),
    ).join("");

    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  }

  return md5(`${Date.now()}-${performance.now()}-${Math.random()}`);
}



export const uploadChunk = <T extends {}>(
  url: string,
  body: Blob,
  params: UploadParams,
  signal: AbortSignal,
  onProgress: (progress: number) => void,
) => new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();

    const uploadUrl = new URL(url);

    for (const key of Object.keys(params)) {
      uploadUrl.searchParams.append(key, String(params[key]));
    }

    signal.addEventListener("abort", () => xhr.abort());

    xhr.open("POST", uploadUrl.href, true);
    xhr.withCredentials = true;
    xhr.setRequestHeader("Content-Type", "application/octet-stream");

    xhr.responseType = "json";

    xhr.upload.addEventListener("progress", (event) => {
      if (event.lengthComputable) {
        onProgress((event.loaded / event.total) * 100);
      }
    });

    xhr.addEventListener("load", () => {
      if (xhr.status < 200 || xhr.status >= 300) {
        const message = xhr.response?.message || xhr.statusText || "upload failed";
        reject(new Error(message));
        return;
      }
      onProgress(100);
      resolve(xhr.response as T);
    });

    xhr.addEventListener("abort", () => {
      reject(new Error("upload aborted"));
    });
    xhr.addEventListener("error", () => {
      reject(new Error("upload failed"));
    });
    xhr.send(body);
  });

export const uploadFile = async (
  file: File,
  path: string,
  chunkSize: number,
  userId: number,
  concurrency: number,
  retries: number,
  retryDelay: number,
  encyptFile: boolean,
  randomChunking: boolean,
  signal: AbortSignal,
  onProgress: (progress: number) => void,
  onChunksCompleted: (chunks: number) => void,
  onCreate: (payload: components["schemas"]["File"]) => Promise<void>,
  skipCheck = false,
  options: UploadFileOptions = {},
) => {
  const fileName = file.name;

  if (!skipCheck && !options.shareId) {
    const res = (
      await fetchClient.GET("/files", {
        params: {
          query: { name: fileName, operation: "find", path },
        },
      })
    ).data;

    if (res && res.items.length > 0) {
      throw new Error("file exists");
    }
  }

  const totalParts = Math.ceil(file.size / chunkSize);

  const limit = pLimit(concurrency);

  const uploadId = options.shareId
    ? generateUUID()
    : md5(
      `${path}/${fileName}${file.size.toString()}${formatTime(file.lastModified)}${userId}`,
    );

  const url = options.shareId
    ? `${window.location.origin}/api/shares/${options.shareId}/uploads/${uploadId}`
    : `${window.location.origin}/api/uploads/${uploadId}`;

  const uploadedParts = options.shareId
    ? []
    : (
      await fetchClient.GET("/uploads/{id}", {
        params: {
          path: {
            id: uploadId,
          },
        },
      })
    ).data!;

  let channelId = 0;

  if (uploadedParts.length > 0) {
    ({ channelId } = uploadedParts[0]);
  }

  const partUploadPromises: Promise<components["schemas"]["UploadPart"]>[] = [];

  const partProgress: number[] = [];

  for (let partIndex = 0; partIndex < totalParts; partIndex++) {
    if (
      uploadedParts?.findIndex((item) => item.partNo === partIndex + 1) > -1
    ) {
      partProgress[partIndex] = 100;
      continue;
    }

    partUploadPromises.push(
      limit(() =>
        (async () => {
          const start = partIndex * chunkSize;

          const end = Math.min(partIndex * chunkSize + chunkSize, file.size);

          const fileBlob = totalParts > 1 ? file.slice(start, end) : file;

          const partName = randomChunking
            ? md5(generateUUID())
            : (totalParts > 1
              ? `${fileName}.part.${zeroPad(partIndex + 1, 3)}`
              : fileName);

          const params = options.shareId
            ? {
              encrypted: encyptFile,
              fileName,
              partNo: partIndex + 1,
            } as const
            : {
              channelId,
              encrypted: encyptFile,
              fileName,
              partName,
              partNo: partIndex + 1,
            } as const;

          let retryCount = 0;
          let asset: components["schemas"]["UploadPart"] | null = null;

          while (retryCount <= retries) {
            try {
              asset = await uploadChunk<components["schemas"]["UploadPart"]>(
                url,
                fileBlob,
                params,
                signal,
                (progress) => {
                  partProgress[partIndex] = progress;
                },
              );
              break;
            } catch (error) {
              if (signal.aborted || retryCount === retries) {
                throw error;
              }
              retryCount++;
              partProgress[partIndex] = 0;
              await new Promise((resolve) =>
                setTimeout(resolve, retryDelay * retryCount),
              );
            }
          }

          return asset!;
        })(),
      ),
    );
  }

  const timer = setInterval(() => {
    const totalProgress = partProgress.reduce(
      (sum, progress) => sum + progress,
      0,
    );
    onProgress(totalParts > 0 ? totalProgress / totalParts : 0);

    const completedChunks = partProgress.filter((p) => p === 100).length;
    onChunksCompleted(completedChunks);
  }, 200);

  signal.addEventListener("abort", () => {
    limit.clearQueue();
    clearInterval(timer);
  });

  try {
    const parts = await Promise.all(partUploadPromises);

    const uploadParts = uploadedParts
      .concat(parts)
      .toSorted((a, b) => a.partNo - b.partNo)
      .map((item) => ({ id: item.partId, salt: item.salt }));

    const basePayload = {
      encrypted: encyptFile,
      mimeType: file.type ?? "application/octet-stream",
      name: fileName,
      path: path ? path : "/",
      size: file.size,
      type: "file",
    } as const;

    const payload = options.shareId
      ? {
        ...basePayload,
        uploadId,
      } as const
      : {
        ...basePayload,
        channelId,
        parts: uploadParts,
      } as const;

    await onCreate(payload);
    if (!options.shareId) {
      await fetchClient.DELETE("/uploads/{id}", {
        params: {
          path: {
            id: uploadId,
          },
        },
      });
    }
    clearInterval(timer);
  } catch (error) {
    clearInterval(timer);
    throw error;
  }
};
