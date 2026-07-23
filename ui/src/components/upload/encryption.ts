export const resolveUploadEncryption = (
  explicitEncryptFiles: boolean | undefined,
  localEncryptFiles: boolean,
) => explicitEncryptFiles ?? localEncryptFiles;
