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
import { sharefileActions, useShareFileAction } from "@/hooks/use-file-action";
import { useModalStore } from "@/utils/stores";
import PreviewModal from "./modals/preview";
import { $api } from "@/utils/api";

const route = getRouteApi("/_share/share/$id");

const disabledActions = [
  FbActions.UploadFiles.id,
  FbActions.CreateFolder.id,
  FbActions.CutFiles.id,
  FbActions.SelectMode.id,
  FbActions.PasteFiles.id,
  FbActions.RenameFile.id,
  FbActions.DeleteFiles.id,
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
    data: { name, type },
  } = $api.useSuspenseQuery("get", "/shares/{id}", {
    params: {
      path: {
        id,
      },
    },
  });

  const { data: files } = useSuspenseInfiniteQuery(shareQueries.list(params));

  const actionHandler = useShareFileAction(params);

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

  return (
    <div className="size-full m-auto">
      <FileBrowser
        files={files}
        folderChain={folderChain}
        onFileAction={actionHandler()}
        fileActions={sharefileActions}
        breakpoint={breakpoint}
        defaultFileViewActionId={defaultViewId}
        disableEssentailFileActions={disabledActions}
      >
        <FileNavbar breakpoint={breakpoint} />
        <FileToolbar className="pt-2" />
        <FileList />
        <FileContextMenu />
      </FileBrowser>
      {modalOperation === FbActions.OpenFiles.id && modalOpen && (
        <PreviewModal shareId={params.id} files={files} view="shared" path="" />
      )}
    </div>
  );
});
