import { memo, useMemo } from "react";
import { useSuspenseInfiniteQuery } from "@tanstack/react-query";
import { getRouteApi } from "@tanstack/react-router";
import {
  FbActions,
  FileBrowser,
  FileContextMenu,
  FileList,
  FileNavbar,
  FileToolbar,
} from "file-browser";
import useBreakpoint from "use-breakpoint";

import { chainSharedLinks } from "@/utils/common";
import { BREAKPOINTS, defaultViewId } from "@/utils/defaults";
import { shareQueries } from "@/utils/query-options";
import { CustomActions, sharefileActions, useShareFileAction } from "@/hooks/use-file-action";
import { useFileUploadStore, useModalStore } from "@/utils/stores";
import PreviewModal from "./modals/preview";
import { $api } from "@/utils/api";
import { FileOperationModal } from "./modals/file-operation";
import { Upload } from "./upload";
import { UploadDropzone } from "./upload/drop-zone";

const route = getRouteApi("/_share/share/$id");

const disabledActions = [
  FbActions.CutFiles.id,
  FbActions.SelectMode.id,
  FbActions.PasteFiles.id,
  FbActions.RenameFile.id,
  FbActions.DeleteFiles.id,
  CustomActions.UploadFolder.id,
];

export const SharedFileBrowser = memo(() => {
  const { id } = route.useParams();

  const { path } = route.useSearch();

  const { breakpoint } = useBreakpoint(BREAKPOINTS);

  const params = {
    id,
    path: path || "",
  };

  const {
    data: { allowUpload, encryptUploads, name, type },
  } = $api.useSuspenseQuery("get", "/shares/{id}", {
    params: {
      path: {
        id,
      },
    },
  });

  const { data: files } = useSuspenseInfiniteQuery(shareQueries.list(params));

  const queryOptions = shareQueries.list(params);

  const actionHandler = useShareFileAction(params);

  const enabledActions = useMemo(() => {
    if (type === "folder" && allowUpload) {
      return disabledActions;
    }
    return [FbActions.UploadFiles.id, FbActions.CreateFolder.id, ...disabledActions];
  }, [allowUpload, type]);

  const folderChain = useMemo(() => {
    if (type === "file") {
      return [];
    }
    return chainSharedLinks(name, params.path!).map(([name, path], index) => ({
      chain: true,
      id: index + name,
      isDir: true,
      name,
      path,
    }));
  }, [params.path, name, type]);

  const modalOpen = useModalStore((state) => state.open);

  const modalOperation = useModalStore((state) => state.operation);

  const openUpload = useFileUploadStore((state) => state.uploadOpen);

  return (
    <div className="size-full m-auto relative">
      <UploadDropzone isDisabled={type !== "folder" || !allowUpload}>
        <FileBrowser
          files={files}
          folderChain={folderChain}
          onFileAction={actionHandler()}
          fileActions={sharefileActions}
          breakpoint={breakpoint}
          defaultFileViewActionId={defaultViewId}
          disableEssentailFileActions={enabledActions}
        >
          <FileNavbar breakpoint={breakpoint} />
          <FileToolbar className="pt-2" />
          <FileList />
          <FileContextMenu />
        </FileBrowser>
      </UploadDropzone>
      {modalOperation === FbActions.CreateFolder.id && modalOpen && (
        <FileOperationModal queryKey={queryOptions.queryKey} mode="share" shareId={params.id} path={params.path} />
      )}
      {modalOperation === FbActions.OpenFiles.id && modalOpen && (
        <PreviewModal shareId={params.id} files={files} view="shared" path="" />
      )}
      {openUpload && (
        <Upload
          queryKey={queryOptions.queryKey}
          mode="share"
          shareId={params.id}
          path={params.path}
          encryptFiles={encryptUploads}
        />
      )}
    </div>
  );
});
