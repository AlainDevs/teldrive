import { useCallback } from "react";
import type { FileListParams, Session, ShareListParams } from "@/types";
import { useQueryClient } from "@tanstack/react-query";
import {
  type FbActionUnion,
  FbActions,
  FbIconName,
  type FileData,
  FileHelper,
  type MapFileActionsToData,
  defineFileAction,
} from "file-browser";
import IconFlatColorIconsVlc from "~icons/flat-color-icons/vlc";
import IconPotPlayerIcon from "~icons/material-symbols/play-circle-rounded";
import toast from "react-hot-toast";

import {
  mediaUrl,
  navigateToExternalUrl,
  sharedMediaUrl,
} from "@/utils/common";
import { SortOrder, getSortState } from "@/utils/defaults";
import { useFileUploadStore, useModalStore } from "@/utils/stores";
import Share from "~icons/fluent/share-24-regular";
import MaterialSymbolsFolder from "~icons/material-symbols/folder";
import { useNavigate } from "@tanstack/react-router";
import { $api } from "@/utils/api";

export const CustomActions = {
  CopyDownloadLink: defineFileAction({
    button: {
      contextMenu: true,
      icon: FbIconName.copy,
      name: "Copy Link",
    },
    fileFilter: (file) => !(file && "isDir" in file),
    id: "copy_link",
    requiresSelection: true,
  } as const),
  OpenInPotPlayer: defineFileAction({
    button: {
      group: "OpenOptions",
      icon: IconPotPlayerIcon,
      name: "PotPlayer",
      toolbar: true,
    },
    fileFilter: (file) => file?.previewType === "video",
    id: "open_pot_player",
    requiresSelection: true,
  } as const),
  OpenInVLCPlayer: defineFileAction({
    button: {
      group: "OpenOptions",
      icon: IconFlatColorIconsVlc,
      name: "VLC",
      toolbar: true,
    },
    fileFilter: (file) => file?.previewType === "video",
    id: "open_vlc_player",
    requiresSelection: true,
  } as const),
  ShareFiles: defineFileAction({
    button: {
      contextMenu: true,
      icon: Share,
      name: "Share",
    },
    id: "share_files",
    requiresSelection: true,
  } as const),
  UploadFolder: defineFileAction({
    button: {
      group: "Add",
      icon: MaterialSymbolsFolder,
      name: "Upload Folder",
      toolbar: true,
    },
    id: "upload_folder",
    requiresSelection: false,
  } as const),
};

type FbActionFullUnion =
  | (typeof CustomActions)[keyof typeof CustomActions]
  | FbActionUnion;

export const useFileAction = (
  { view, params: search }: FileListParams,
  session: Session,
) => {
  const queryClient = useQueryClient();

  const actions = useModalStore((state) => state.actions);

  const fileDialogOpen = useFileUploadStore(
    (state) => state.actions.setFileDialogOpen,
  );

  const setFolderDialogOpen = useFileUploadStore(
    (state) => state.actions.setFolderDialogOpen,
  );

  const uploadOpen = useFileUploadStore((state) => state.actions.setUploadOpen);

  const navigate = useNavigate();

  const moveFiles = $api.useMutation("post", "/files/move");

  return useCallback(() => async (data: MapFileActionsToData<FbActionFullUnion>) => {
      switch (data.id) {
        case FbActions.OpenFiles.id: {
          const { targetFile, files } = data.payload;

          const fileToOpen = targetFile ?? files[0];

          if (fileToOpen && FileHelper.isDirectory(fileToOpen)) {
            let qparams: FileListParams;

            if (view === "my-drive") {
              const basePath = search?.path ?? "/";
              qparams = {
                params: {
                  path: fileToOpen.chain
                    ? fileToOpen.path
                    : `${basePath === "/" ? "" : basePath}/${fileToOpen.name}`,
                },
                view,
              };
            } else {
              qparams = {
                params: { parentId: fileToOpen.id },
                view: "browse",
              };
            }
            navigate({
              params: { view: qparams.view },
              search: qparams.params,
              to: "/$view",
            });
          } else if (fileToOpen && FileHelper.isOpenable(fileToOpen)) {
            actions.set({
              currentFile: fileToOpen,
              open: true,
              operation: FbActions.OpenFiles.id,
            });
          }

          break;
        }
        case FbActions.DownloadFiles.id: {
          const { selectedFiles } = data.state;
          for (const file of selectedFiles) {
            if (!FileHelper.isDirectory(file)) {
              const { id, name } = file;
              const url = mediaUrl(
                id,
                name,
                search?.path || "",
                session.sessionId,
                true,
              );
              navigateToExternalUrl(url, false);
            }
          }
          break;
        }
        case CustomActions.OpenInVLCPlayer.id: {
          const { selectedFiles } = data.state;
          const fileToOpen = selectedFiles[0];
          const { id, name } = fileToOpen!;
          const url = `vlc://${mediaUrl(id, name, search?.path || "", session.sessionId)}`;
          navigateToExternalUrl(url, false);
          break;
        }
        case CustomActions.OpenInPotPlayer.id: {
          const { selectedFiles } = data.state;
          const fileToOpen = selectedFiles[0];
          const { id, name } = fileToOpen!;
          const url = `potplayer://${mediaUrl(id, name, search?.path || "", session.sessionId)}`;
          navigateToExternalUrl(url, false);
          break;
        }
        case FbActions.RenameFile.id: {
          actions.set({
            currentFile: data.state.selectedFiles[0],
            open: true,
            operation: FbActions.RenameFile.id,
          });
          break;
        }
        case FbActions.DeleteFiles.id: {
          actions.set({
            open: true,
            operation: FbActions.DeleteFiles.id,
            selectedFiles: data.state.selectedFiles.map((item) => item.id),
          });
          break;
        }
        case FbActions.CreateFolder.id: {
          actions.set({
            currentFile: {} as FileData,
            open: true,
            operation: FbActions.CreateFolder.id,
          });
          break;
        }

        case CustomActions.ShareFiles.id: {
          actions.set({
            currentFile: data.state.selectedFiles[0],
            open: true,
            operation: CustomActions.ShareFiles.id,
          });
          break;
        }

        case CustomActions.CopyDownloadLink.id: {
          const selections = data.state.selectedFilesForAction;
          const clipboardText = selections
            .filter((element) => !FileHelper.isDirectory(element))
            .map(({ id, name }) =>
              mediaUrl(id, name, search?.path || "", session.sessionId, true),
            )
            .join("\n");
          navigator.clipboard.writeText(clipboardText);
          break;
        }
        case FbActions.MoveFiles.id: {
          const { files, target } = data.payload;
          moveFiles
            .mutateAsync({
              body: {
                destinationParent: target.path || "/",
                ids: files.map((file) => file?.id!),
              },
            })
            .then(() => {
              toast.success(`${files.length} files moved successfully`);
              queryClient.invalidateQueries({ queryKey: ["Files_list", "my-drive"] });
            });

          break;
        }

        case FbActions.UploadFiles.id: {
          fileDialogOpen(true);
          setFolderDialogOpen(false);
          uploadOpen(true);
          break;
        }

        case CustomActions.UploadFolder.id: {
          fileDialogOpen(false);
          setFolderDialogOpen(true);
          uploadOpen(true);
          break;
        }

        case FbActions.EnableListView.id:
        case FbActions.EnableGridView.id: {
          localStorage.setItem("viewId", data.id);
          break;
        }
        case FbActions.SortFilesByName.id:
        case FbActions.SortFilesBySize.id:
        case FbActions.SortFilesByDate.id: {
          if (view === "my-drive") {
            const currentSortState = getSortState();
            const order =
              currentSortState.order === SortOrder.ASC
                ? SortOrder.DESC
                : SortOrder.ASC;
            localStorage.setItem(
              "sort",
              JSON.stringify({ order, sortId: data.id }),
            );
          }
          break;
        }
        default:
          break;
      }
    }, [view, search?.path]);
};

export const useShareFileAction = (params: ShareListParams) => {
  const actions = useModalStore((state) => state.actions);
  const navigate = useNavigate();
  return useCallback(() => async (data: MapFileActionsToData<FbActionFullUnion>) => {
      switch (data.id) {
        case FbActions.OpenFiles.id: {
          const { targetFile, files } = data.payload;

          const fileToOpen = targetFile ?? files[0];

          if (fileToOpen && FileHelper.isDirectory(fileToOpen)) {
            const basePath = params?.path ?? "/";
            navigate({
              params: {
                id: params.id,
              },
              search: {
                path: fileToOpen.chain
                  ? fileToOpen.path
                  : `${basePath === "/" ? "" : basePath}/${fileToOpen.name}`,
              },
              to: "/share/$id",
            });
          } else if (fileToOpen && FileHelper.isOpenable(fileToOpen)) {
            actions.set({
              currentFile: fileToOpen,
              open: true,
              operation: FbActions.OpenFiles.id,
            });
          }

          break;
        }
        case FbActions.DownloadFiles.id: {
          const { selectedFiles } = data.state;
          for (const file of selectedFiles) {
            if (!FileHelper.isDirectory(file)) {
              const { id } = file;
              const url = sharedMediaUrl(params.id, id, true);
              navigateToExternalUrl(url, false);
            }
          }
          break;
        }
        case CustomActions.OpenInVLCPlayer.id: {
          const { selectedFiles } = data.state;
          const fileToOpen = selectedFiles[0];
          const { id } = fileToOpen!;
          const url = `vlc://${sharedMediaUrl(params.id, id)}`;
          navigateToExternalUrl(url, false);
          break;
        }
        case CustomActions.OpenInPotPlayer.id: {
          const { selectedFiles } = data.state;
          const fileToOpen = selectedFiles[0];
          const { id } = fileToOpen!;
          const url = `potplayer://${sharedMediaUrl(params.id, id)}`;
          navigateToExternalUrl(url, false);
          break;
        }

        case CustomActions.CopyDownloadLink.id: {
          const selections = data.state.selectedFilesForAction;
          const clipboardText = selections
            .filter((element) => !FileHelper.isDirectory(element))
            .map(({ id }) => sharedMediaUrl(params.id, id, true))
            .join("\n");
          navigator.clipboard.writeText(clipboardText);
          break;
        }

        case FbActions.EnableListView.id:
        case FbActions.EnableGridView.id: {
          localStorage.setItem("viewId", data.id);
          break;
        }
        default:
          break;
      }
    }, [params.path, params.id]);
};

export const fileActions = Object.keys(CustomActions).map(
    (t) => CustomActions[t as keyof typeof CustomActions],
  );

export const sharefileActions = Object.keys(CustomActions)
  .map((t) => CustomActions[t as keyof typeof CustomActions])
  .filter((action) => action.id !== CustomActions.ShareFiles.id);
