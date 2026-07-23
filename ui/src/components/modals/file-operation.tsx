import { memo, useCallback, useEffect, useState } from "react";
import { FbActions } from "file-browser";
import {
  Button,
  Input,
  Modal,
  Separator,
  Switch,
} from "@heroui/react";
import { useShallow } from "zustand/react/shallow";

import { useModalStore } from "@/utils/stores";
import { Controller, useForm } from "react-hook-form";
import { CustomActions } from "@/hooks/use-file-action";
import { CopyButton } from "@/components/copy-button";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import IcRoundClose from "~icons/ic/round-close";
import { getNextDate } from "@/utils/common";
import ShowPasswordIcon from "~icons/mdi/eye-outline";
import HidePasswordIcon from "~icons/mdi/eye-off-outline";
import MdiProtectedOutline from "~icons/mdi/protected-outline";
import { $api } from "@/utils/api";
import { deleteConfirmation } from "@/utils/delete-files";

interface FileModalProps {
  queryKey: any;
  mode?: "drive" | "share";
  shareId?: string;
  path?: string;
}

interface RenameDialogProps {
  queryKey: any;
  handleClose: () => void;
}

const RenameDialog = memo(({ queryKey, handleClose }: RenameDialogProps) => {
  const queryClient = useQueryClient();
  const updateFiles = $api.useMutation("patch", "/files/{id}", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });
  const { currentFile, actions } = useModalStore(
    useShallow((state) => ({
      actions: state.actions,
      currentFile: state.currentFile,
    })),
  );

  const onRename = useCallback(
    (e: React.FormEvent<HTMLFormElement>) => {
      e.preventDefault();
      updateFiles
        .mutateAsync({
          body: {
            name: currentFile?.name,
          },
          params: {
            path: {
              id: currentFile.id,
            },
          },
        })
        .then(handleClose);
    },
    [currentFile.name, currentFile.id],
  );

  return (
    <>
      <Modal.Header className="flex flex-col gap-1">
        <Modal.Heading>Rename</Modal.Heading>
      </Modal.Header>
      <Modal.Body>
        <form id="rename-form" onSubmit={onRename}>
          <Input
            className="border-large"
            autoFocus
            value={currentFile.name}
            onChange={(e) => actions.setCurrentFile({ ...currentFile, name: e.target.value })}
          />
        </form>
      </Modal.Body>
      <Modal.Footer>
        <Button className="font-normal" variant="ghost" onPress={handleClose}>
          Close
        </Button>
        <Button
          type="submit"
          className="font-normal"
          variant="secondary"
          form="rename-form"
          isDisabled={updateFiles.isPending || !currentFile.name}
        >
          Rename
        </Button>
      </Modal.Footer>
    </>
  );
});

interface FolderCreateDialogProps {
  queryKey: any;
  handleClose: () => void;
  mode?: "drive" | "share";
  shareId?: string;
  path?: string;
}

const FolderCreateDialog = memo(({ queryKey, handleClose, mode = "drive", shareId, path }: FolderCreateDialogProps) => {
  const queryClient = useQueryClient();

  const createFolder = $api.useMutation("post", "/files", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  const createShareFolder = $api.useMutation("post", "/shares/{id}/files", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });
  const { currentFile, actions } = useModalStore(
    useShallow((state) => ({
      actions: state.actions,
      currentFile: state.currentFile,
    })),
  );

  const onCreate = useCallback(
    (e: React.FormEvent<HTMLFormElement>) => {
      e.preventDefault();
      const body = {
        name: currentFile.name,
        path: path || "/",
        type: "folder" as const,
      };
      const request = mode === "share" && shareId
        ? createShareFolder.mutateAsync({
          body,
          params: {
            path: {
              id: shareId,
            },
          },
        })
        : createFolder.mutateAsync({
          body: {
            name: currentFile.name,
            path: path || "/",
            type: "folder",
          },
        });
      request
        .then(() => handleClose());
    },
    [currentFile.name, mode, path, shareId],
  );

  const isPending = mode === "share" ? createShareFolder.isPending : createFolder.isPending;

  return (
    <>
      <Modal.Header className="flex flex-col gap-1">
        <Modal.Heading>Create Folder</Modal.Heading>
      </Modal.Header>
      <Modal.Body>
        <form id="create-folder-form" onSubmit={onCreate}>
          <Input
            className="border-large"
            placeholder="Folder Name or Path"
            autoFocus
            value={currentFile?.name}
            onChange={(e) => actions.setCurrentFile({ ...currentFile, name: e.target.value })}
          />
        </form>
      </Modal.Body>
      <Modal.Footer>
        <Button className="font-normal" variant="ghost" onPress={handleClose}>
          Close
        </Button>
        <Button
          type="submit"
          className="font-normal"
          variant="secondary"
          form="create-folder-form"
          isDisabled={isPending || !currentFile.name}
        >
          {isPending ? "Creating" : "Create"}
        </Button>
      </Modal.Footer>
    </>
  );
});

interface DeleteDialogProps {
  handleClose: () => void;
}

const DeleteDialog = memo(({ handleClose }: DeleteDialogProps) => {
  const queryClient = useQueryClient();

  const deleteFiles = $api.useMutation("post", "/files/delete", {
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["Files_list"] });
    },
  });

  const selectedFiles = useModalStore((state) => state.selectedFiles);

  const onDelete = useCallback(async () => {
    await deleteFiles.mutateAsync({ body: { ids: selectedFiles.map((file) => file.id) } });
    handleClose();
  }, [deleteFiles, handleClose, selectedFiles]);

  return (
    <>
      <Modal.Header className="flex flex-col gap-1">
        <Modal.Heading>Delete Files</Modal.Heading>
      </Modal.Header>
      <Modal.Body>
        <h1 className="text-large font-medium mt-2">
          {deleteConfirmation(selectedFiles)}
        </h1>
      </Modal.Body>
      <Modal.Footer>
        <Button className="font-normal" variant="ghost" onPress={handleClose}>
          No
        </Button>
        <Button
          variant="secondary"
          className="font-normal"
          onPress={onDelete}
        >
          Yes
        </Button>
      </Modal.Footer>
    </>
  );
});

interface ShareFileDialogProps {
  handleClose: () => void;
}

const defaultShareOptions = {
  allowUpload: false,
  expirationDate: "",
  password: "",
};

type ShareFormOptions = typeof defaultShareOptions;

type SharePayload = {
  allowUpload?: boolean;
  password?: string;
  expiresAt?: string;
};

type ShareEntry = {
  id: string;
  protected: boolean;
  allowUpload?: boolean;
};

const ShareFileDialog = memo(({ handleClose }: ShareFileDialogProps) => {
  const file = useModalStore((state) => state.currentFile);

  const queryClient = useQueryClient();

  const { control, handleSubmit, reset } = useForm<ShareFormOptions>({
    defaultValues: defaultShareOptions,
  });

  const shareQueryOptions = $api.queryOptions("get", "/files/{id}/shares", {
    params: {
      path: {
        id: file.id,
      },
    },
  });

  const { data, isLoading } = useQuery(shareQueryOptions);

  const createShare = $api.useMutation("post", "/files/{id}/shares", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: shareQueryOptions.queryKey });
      queryClient.invalidateQueries({ queryKey: ["Files_list", "shared"] });
    },
  });

  const editShare = $api.useMutation("patch", "/files/{id}/shares/{shareId}", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: shareQueryOptions.queryKey });
      queryClient.invalidateQueries({ queryKey: ["Files_list", "shared"] });
    },
  });

  const deleteShare = $api.useMutation("delete", "/files/{id}/shares/{shareId}", {
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["Files_list", "shared"] });
    },
  });

  const [sharingOn, setSharingOn] = useState(false);

  const [shareLink, setShareLink] = useState("");

  const [showPassword, setShowPassword] = useState(false);

  const onShareChange = useCallback(() => {
    setSharingOn((prev) => {
      if (!prev) {
        handleSubmit((data) => {
          const payload = {} as SharePayload;
          if (data.expirationDate) {
            payload.expiresAt = `${data.expirationDate}${new Date().toISOString().slice(10)}`;
          }
          if (data.password) {
            payload.password = data.password;
          }
          if (file.type === "folder") {
            payload.allowUpload = data.allowUpload;
          }
          createShare.mutateAsync({
            body: payload,
            params: {
              path: {
                id: file.id,
              },
            },
          });
        })();
      }
      if (prev) {
        const shareId = data?.[0]?.id;
        if (!shareId) {
          setShareLink("");
          return !prev;
        }
        deleteShare.mutateAsync({
          params: {
            path: {
              id: file.id,
              shareId,
            },
          },
        });
        setShareLink("");
      }
      return !prev;
    });
  }, [createShare, data, deleteShare, file.id, file.type, handleSubmit]);

  const onAllowUploadChange = useCallback((allowUpload: boolean) => {
    const shareId = data?.[0]?.id;
    if (!shareId || file.type !== "folder") {
      return;
    }
    editShare.mutateAsync({
      body: { allowUpload } as SharePayload,
      params: {
        path: {
          id: file.id,
          shareId,
        },
      },
    });
  }, [data, editShare, file.id, file.type]);

  useEffect(() => {
    if (data && data.length > 0) {
      const share = data[0] as ShareEntry;
      setSharingOn(true);
      reset((currentValues) => ({
        ...currentValues,
        allowUpload: Boolean(share.allowUpload),
      }));
      setShareLink(`${window.location.origin}/share/${share.id}`);
    }
  }, [data, reset]);

  return (
    <>
      <Modal.Header className="flex items-center justify-between ">
        <Modal.Heading>Share Files</Modal.Heading>
        <Button size="sm" variant="ghost" isIconOnly onPress={handleClose}>
          <IcRoundClose />
        </Button>
      </Modal.Header>
      <Modal.Body>
        <form className="grid grid-cols-6 gap-8 p-2 w-full overflow-y-auto">
          <div className="col-span-6 xs:col-span-3">
            <p className="text-lg font-medium">Set expiration date</p>
            <p className="text-sm font-normal text-muted">Link expiration date</p>
          </div>
          <Controller
            name="expirationDate"
            control={control}
            render={({ field }) => (
              <Input
                className="col-span-6 xs:col-span-3"
                type="date"
                min={getNextDate()}
                {...field}
              />
            )}
          />
          <div className="col-span-6 xs:col-span-3">
            <p className="text-lg font-medium">Set link password</p>
            <p className="text-sm font-normal text-muted">Public link password</p>
          </div>
          <Controller
            name="password"
            control={control}
            render={({ field }) => (
              <div className="col-span-6 xs:col-span-3 relative">
                <Input
                  className="col-span-6 xs:col-span-3"
                  autoComplete="off"
                  type={showPassword ? "text" : "password"}
                  {...field}
                />
                <Button
                  isIconOnly
                  className="size-8 min-w-8 absolute right-2 top-1/2 -translate-y-1/2 z-10"
                  variant="ghost"
                  onPress={() => setShowPassword((prev) => !prev)}
                >
                  {showPassword ? <HidePasswordIcon /> : <ShowPasswordIcon />}
                </Button>
              </div>
            )}
          />
          {file.type === "folder" && (
            <>
              <div className="col-span-6 xs:col-span-3">
                <p className="text-lg font-medium">Allow uploads and folder creation</p>
                <p className="text-sm font-normal text-muted">Recipients can add files and folders to this share</p>
              </div>
              <Controller
                name="allowUpload"
                control={control}
                render={({ field }) => (
                  <Switch
                    className="col-span-6 xs:col-span-3 justify-self-start"
                    isSelected={field.value}
                    isDisabled={editShare.isPending}
                    onChange={(isSelected) => {
                      field.onChange(isSelected);
                      if (sharingOn) {
                        onAllowUploadChange(isSelected);
                      }
                    }}
                    size="md"
                    aria-label="Allow uploads and folder creation"
                  >
                    <Switch.Control>
                      <Switch.Thumb />
                    </Switch.Control>
                  </Switch>
                )}
              />
            </>
          )}
        </form>
        <Separator />
        <div className="flex justify-between">
          <h1 className="text-large font-medium mt-2">Sharing {sharingOn ? "On" : "Off"}</h1>
          <div className="flex items-center gap-3">
            {data?.[0]?.protected && <MdiProtectedOutline className="text-accent" />}

            <Switch isSelected={sharingOn} onChange={onShareChange} size="md" aria-label="Toggle sharing">
              <Switch.Control>
                <Switch.Thumb />
              </Switch.Control>
            </Switch>
          </div>
        </div>
      </Modal.Body>
      <Modal.Footer>
        <Input
          disabled={isLoading || !data || data.length === 0}
          fullWidth
          readOnly
          value={shareLink}
        />
        <CopyButton value={shareLink} isDisabled={isLoading || !data || data.length === 0} />
      </Modal.Footer>
    </>
  );
});

export const FileOperationModal = memo(({ queryKey, mode = "drive", shareId, path }: FileModalProps) => {
  const { open, operation, actions } = useModalStore(
    useShallow((state) => ({
      actions: state.actions,
      open: state.open,
      operation: state.operation,
    })),
  );

  const handleClose = useCallback(
    () =>
      actions.set({
        open: false,
      }),
    [],
  );

  const renderOperation = () => {
    switch (operation) {
      case FbActions.RenameFile.id:
        return <RenameDialog queryKey={queryKey} handleClose={handleClose} />;
      case FbActions.CreateFolder.id:
        return <FolderCreateDialog queryKey={queryKey} handleClose={handleClose} mode={mode} shareId={shareId} path={path} />;
      case FbActions.DeleteFiles.id:
        return <DeleteDialog handleClose={handleClose} />;
      case CustomActions.ShareFiles.id:
        return <ShareFileDialog handleClose={handleClose} />;
      default:
        return null;
    }
  };

  return (
    <Modal.Backdrop isOpen={open} onOpenChange={(isOpen) => { if (!isOpen) {handleClose();} }}>
      <Modal.Container>
        <Modal.Dialog>
          {renderOperation()}
        </Modal.Dialog>
      </Modal.Container>
    </Modal.Backdrop>
  );
});
