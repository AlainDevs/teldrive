import type { FileData } from "file-browser";

export const deleteTargets = (selectedFilesForAction: FileData[]) =>
  selectedFilesForAction;

export const deleteConfirmation = (files: FileData[]) => {
  if (files.length === 1 && files[0]?.isDir) {
    return `Are you sure you want to delete folder "${files[0].name}" and all its contents?`;
  }

  const folderCount = files.filter((file) => file.isDir).length;
  const fileCount = files.length - folderCount;
  if (folderCount > 0) {
    return `Are you sure you want to delete ${fileCount} file${fileCount === 1 ? "" : "s"} and ${folderCount} folder${folderCount === 1 ? "" : "s"}, including all folder contents?`;
  }

  return `Are you sure you want to delete ${fileCount} file${fileCount === 1 ? "" : "s"}?`;
};
