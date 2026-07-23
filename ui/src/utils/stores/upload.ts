import { create } from "zustand";
import { immer } from "zustand/middleware/immer";

export enum FileUploadStatus {
  NOT_STARTED = 0,
  UPLOADING = 1,
  UPLOADED = 2,
  CANCELLED = 3,
  FAILED = 4,
  SKIPPED = 5,
}

export interface UploadFile {
  id: string;
  file: File;
  status: FileUploadStatus;
  totalChunks: number;
  controller: AbortController;
  progress: number;
  relativePath?: string;
  parentFolderId?: string;
  isFolder: boolean;
  folderId?: string;
  speed?: number;
  eta?: number;
  chunksCompleted?: number;
  error?: string;
  collapsed?: boolean;
}

import { scanEntries } from "../file-scanner";

export interface UploadState {
  filesIds: string[];
  fileMap: Record<string, UploadFile>;
  currentFileId: string;
  collapse: boolean;
  fileDialogOpen: boolean;
  folderDialogOpen: boolean;
  uploadOpen: boolean;
  actions: {
    addFiles: (files: File[]) => void;
    addFolder: (files: File[], folderName: string) => void;
    handleDragDrop: (items: DataTransferItemList) => Promise<void>;
    handleSelection: (files: FileList | null) => void;
    setCurrentFileId: (id: string) => void;
    toggleCollapse: () => void;
    setFileUploadStatus: (id: string, status: FileUploadStatus) => void;
    removeFile: (id: string) => void;
    cancelUpload: () => void;
    setFileDialogOpen: (open: boolean) => void;
    setFolderDialogOpen: (open: boolean) => void;
    setUploadOpen: (open: boolean) => void;
    setProgress: (id: string, progress: number) => void;
    setSpeed: (id: string, speed: number) => void;
    setETA: (id: string, eta: number) => void;
    setChunksCompleted: (id: string, chunks: number) => void;
    setError: (id: string, error: string) => void;
    setFolderId: (id: string, folderId: string) => void;
    toggleFolderCollapsed: (id: string) => void;
    startNextUpload: () => void;
    clearAll: () => void;
  };
}

export const useFileUploadStore = create<UploadState>()(
  immer((set, get) => ({
    actions: {
      addFiles: (files: File[]) =>
        set((state) => {
          const newFiles = files.map((file) => ({
            collapsed: false,
            controller: new AbortController(),
            file,
            id: Math.random().toString(36).slice(2, 9),
            isFolder: false,
            progress: 0,
            speed: 0,
            status: FileUploadStatus.NOT_STARTED,
            totalChunks: 0,
          }));

          const ids = newFiles.map((file) => {
            state.fileMap[file.id] = file;
            return file.id;
          });
          state.filesIds.push(...ids);
          if (!state.currentFileId) {
            state.currentFileId = ids[0];
          } else {
            const currentFile = state.fileMap[state.currentFileId];
            // Update currentFileId if current file is not actively uploading
            const isCurrentFileActive =
              currentFile.status === FileUploadStatus.NOT_STARTED ||
              currentFile.status === FileUploadStatus.UPLOADING;
            if (!isCurrentFileActive) {
              state.currentFileId = ids[0];
            }
          }
        }),

      addFolder: (files: File[], folderName: string) =>
        set((state) => {
          const rootFolderId = Math.random().toString(36).slice(2, 9);
          const rootFolderFile = new File([], folderName, { type: "folder" });

          state.fileMap[rootFolderId] = {
            collapsed: false,
            controller: new AbortController(),
            file: rootFolderFile,
            id: rootFolderId,
            isFolder: true,
            progress: 0,
            relativePath: folderName,
            speed: 0,
            status: FileUploadStatus.NOT_STARTED,
            totalChunks: 0,
          };

          state.filesIds.push(rootFolderId);
          const folderPathMap = new Map<string, string>();
          folderPathMap.set(folderName, rootFolderId);

          files.forEach((file) => {
            const pathParts = file.webkitRelativePath.split("/");
            let currentPath = pathParts[0];
            let parentId = rootFolderId;

            // Process intermediate folders
            for (let i = 1; i < pathParts.length - 1; i++) {
              const part = pathParts[i];
              currentPath = `${currentPath}/${part}`;

              if (!folderPathMap.has(currentPath)) {
                const folderId = Math.random().toString(36).slice(2, 9);
                const folderFile = new File([], part, { type: "folder" });

                state.fileMap[folderId] = {
                  collapsed: false,
                  controller: new AbortController(),
                  file: folderFile,
                  id: folderId,
                  isFolder: true,
                  parentFolderId: parentId,
                  progress: 0,
                  relativePath: currentPath,
                  speed: 0,
                  status: FileUploadStatus.NOT_STARTED,
                  totalChunks: 0,
                };
                state.filesIds.push(folderId);
                folderPathMap.set(currentPath, folderId);
              }
              parentId = folderPathMap.get(currentPath)!;
            }

            // Create file entry
            const fileId = Math.random().toString(36).slice(2, 9);
            state.fileMap[fileId] = {
              collapsed: false,
              controller: new AbortController(),
              file: file,
              id: fileId,
              isFolder: false,
              parentFolderId: parentId,
              progress: 0,
              relativePath: file.webkitRelativePath,
              speed: 0,
              status: FileUploadStatus.NOT_STARTED,
              totalChunks: 0,
            };
            state.filesIds.push(fileId);
          });

          if (!state.currentFileId) {
            state.currentFileId = rootFolderId;
          }
        }),

      cancelUpload: () =>
        set((state) => {
          const file = state.fileMap[state.currentFileId];
          if (file?.controller) {
            file.controller.abort();
          }
          state.fileMap = {};
          state.filesIds = [];
          state.currentFileId = "";
          state.collapse = false;
          state.uploadOpen = false;
          state.fileDialogOpen = false;
          state.folderDialogOpen = false;
        }),

      clearAll: () =>
        set((state) => {
          const completedIds = state.filesIds.filter(
            (id) =>
              state.fileMap[id]?.status === FileUploadStatus.UPLOADED ||
              state.fileMap[id]?.status === FileUploadStatus.CANCELLED ||
              state.fileMap[id]?.status === FileUploadStatus.FAILED ||
              state.fileMap[id]?.status === FileUploadStatus.SKIPPED,
          );
          completedIds.forEach((id) => {
            delete state.fileMap[id];
          });
          state.filesIds = state.filesIds.filter((id) => !completedIds.includes(id));
          if (state.filesIds.length === 0) {
            state.currentFileId = "";
            state.collapse = false;
            state.uploadOpen = false;
          }
        }),

      handleDragDrop: async (items: DataTransferItemList) => {
        const rootFiles: File[] = [];
        const folderOperations: Promise<void>[] = [];
        const { addFiles, addFolder, setUploadOpen } = get().actions;

        for (let i = 0; i < items.length; i++) {
          const entry = items[i].webkitGetAsEntry();
          if (!entry) {continue;}

          if (entry.isDirectory) {
            folderOperations.push(
              (async () => {
                try {
                  const files = await scanEntries(entry);
                  addFolder(files, entry.name);
                } catch (error) {
                  console.error("Folder processing failed:", error);
                }
              })(),
            );
          } else {
            folderOperations.push(
              (async () => {
                try {
                  const file = await new Promise<File>((resolve, reject) =>
                    (entry as FileSystemFileEntry).file(resolve, reject),
                  );
                  rootFiles.push(file);
                } catch (error) {
                  console.error("File processing failed:", error);
                }
              })(),
            );
          }
        }

        await Promise.all(folderOperations);

        if (rootFiles.length > 0) {
          // Check if any files are already being added (by checking fileMap/ids)
          // Or just rely on addFiles to handle it.
          addFiles(rootFiles);
        }

        if (folderOperations.length > 0 || rootFiles.length > 0) {
          setUploadOpen(true);
        }
      },

      handleSelection: (files: FileList | null) => {
        if (!files || files.length === 0) {return;}
        const { addFolder, addFiles } = get().actions;

        const fileList = [...files];
        const firstFile = fileList[0];
        const relativePath = firstFile?.webkitRelativePath;
        const hasFolderStructure = relativePath && fileList.length > 0;

        if (hasFolderStructure) {
          const pathParts = relativePath.split("/");
          const folderName = pathParts[0] || "Untitled Folder";
          addFolder(fileList, folderName);
        } else {
          const validFiles = fileList.filter((f) => f.size > 0);
          if (validFiles.length > 0) {
            addFiles(validFiles);
          }
        }
      },

      removeFile: (id: string) =>
        set((state) => {
          const filesToCancel = [id];
          let i = 0;
          while (i < filesToCancel.length) {
            const currentId = filesToCancel[i];
            const file = state.fileMap[currentId];
            if (file?.isFolder) {
              const children = state.filesIds.filter(
                (fid) => state.fileMap[fid]?.parentFolderId === currentId,
              );
              filesToCancel.push(...children);
            }
            i++;
          }

          const isCurrentFileCancelled = filesToCancel.includes(state.currentFileId);

          filesToCancel.forEach((cancelId) => {
            const file = state.fileMap[cancelId];
            if (file) {
              if (file.controller && file.status !== FileUploadStatus.CANCELLED) {
                file.controller.abort();
              }
              file.status = FileUploadStatus.CANCELLED;
            }
          });

          if (state.filesIds.length === 0) {
            state.currentFileId = "";
            state.collapse = false;
            state.uploadOpen = false;
            state.fileDialogOpen = false;
            state.folderDialogOpen = false;
          } else if (isCurrentFileCancelled) {
            const eligibleFiles = state.filesIds.filter((id) => {
              const file = state.fileMap[id];
              if (file.status !== FileUploadStatus.NOT_STARTED) {return false;}
              if (file.parentFolderId) {
                const parent = state.fileMap[file.parentFolderId];
                return (
                  parent &&
                  (parent.status === FileUploadStatus.UPLOADED ||
                    parent.status === FileUploadStatus.SKIPPED)
                );
              }
              return true;
            });
            state.currentFileId = eligibleFiles[0] || "";
          }
        }),

      setChunksCompleted: (id: string, chunks: number) =>
        set((state) => {
          if (!state.fileMap[id]) {return;}
          state.fileMap[id].chunksCompleted = chunks;
        }),

      setCurrentFileId: (id: string) =>
        set((state) => {
          state.currentFileId = id;
        }),

      setETA: (id: string, eta: number) =>
        set((state) => {
          if (!state.fileMap[id]) {return;}
          state.fileMap[id].eta = eta;
        }),

      setError: (id: string, error: string) =>
        set((state) => {
          if (!state.fileMap[id]) {return;}
          state.fileMap[id].error = error;
        }),

      setFileDialogOpen: (open: boolean) =>
        set((state) => {
          state.fileDialogOpen = open;
        }),

      setFileUploadStatus: (id: string, status: FileUploadStatus) =>
        set((state) => {
          if (!state.fileMap[id]) {return;}
          state.fileMap[id].status = status;
        }),

      setFolderDialogOpen: (open: boolean) =>
        set((state) => {
          state.folderDialogOpen = open;
        }),

      setFolderId: (id: string, folderId: string) =>
        set((state) => {
          if (!state.fileMap[id]) {return;}
          state.fileMap[id].folderId = folderId;
        }),

      setProgress: (id: string, progress: number) =>
        set((state) => {
          if (!state.fileMap[id]) {return;}
          state.fileMap[id].progress = progress;
        }),

      setSpeed: (id: string, speed: number) =>
        set((state) => {
          if (!state.fileMap[id]) {return;}
          state.fileMap[id].speed = speed;
        }),

      setUploadOpen: (open: boolean) =>
        set((state) => {
          state.uploadOpen = open;
        }),

      startNextUpload: () =>
        set((state) => {
          const eligibleFiles = state.filesIds.filter((id) => {
            const file = state.fileMap[id];
            // Skip if already processed
            if (file.status !== FileUploadStatus.NOT_STARTED) {return false;}

            // If file has parent folder, check if parent is uploaded/skipped
            if (file.parentFolderId) {
              const parent = state.fileMap[file.parentFolderId];
              return (
                parent &&
                (parent.status === FileUploadStatus.UPLOADED ||
                  parent.status === FileUploadStatus.SKIPPED)
              );
            }

            return true; // No dependencies, eligible
          });

          state.currentFileId = eligibleFiles[0] || "";
        }),

      toggleCollapse: () =>
        set((state) => {
          state.collapse = !state.collapse;
        }),

      toggleFolderCollapsed: (id: string) =>
        set((state) => {
          if (state.fileMap[id]?.isFolder) {
            state.fileMap[id].collapsed = !state.fileMap[id].collapsed;
          }
        }),
    },
    collapse: false,
    currentFileId: "",
    fileDialogOpen: false,
    fileMap: {},
    filesIds: [],
    folderDialogOpen: false,
    uploadOpen: false,
  })),
);
