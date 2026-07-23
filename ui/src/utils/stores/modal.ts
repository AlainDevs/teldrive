import type { FileData } from "file-browser";
import { create } from "zustand";

interface ModalState {
  open: boolean;
  operation: string;
  type: string;
  currentFile: FileData;
  selectedFiles: FileData[];
  name?: string;
  actions: {
    setOperation: (operation: string) => void;
    setOpen: (open: boolean) => void;
    setCurrentFile: (currentFile: FileData) => void;
    setSelectedFiles: (selectedFiles: FileData[]) => void;
    set: (payload: Partial<ModalState>) => void;
  };
}

export const useModalStore = create<ModalState>((set) => ({
  actions: {
    set: (payload) => set((state) => ({ ...state, ...payload })),
    setCurrentFile: (currentFile: FileData) => set((state) => ({ ...state, currentFile })),
    setOpen: (open: boolean) => set((state) => ({ ...state, open })),
    setOperation: (operation: string) => set((state) => ({ ...state, operation })),
    setSelectedFiles: (selectedFiles: FileData[]) => set((state) => ({ ...state, selectedFiles })),
  },
  currentFile: {} as FileData,
  name: "",
  open: false,
  operation: "",
  selectedFiles: [],
  type: "",
}));
