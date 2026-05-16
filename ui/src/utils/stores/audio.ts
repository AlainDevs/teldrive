import type { Tags } from "@/types";
import { create } from "zustand";
import { parseAudioMetadata } from "@/utils/tagparser";

type AudioElem = HTMLAudioElement | null;

interface AudioMetadata {
  artist: string;
  title: string;
  cover: string;
}

interface AudioHandlers {
  nextItem: (type: string) => void;
  prevItem: (type: string) => void;
}

interface PlayerState {
  audio: AudioElem;
  isPlaying: boolean;
  duration: number;
  isLooping: boolean;
  isMuted: boolean;
  volume: number;
  isEnded: boolean;
  currentTime: number;
  metadata: AudioMetadata;
  metadataController: AbortController | null;
  error: string;
  handlers: AudioHandlers;
  actions: {
    seek: (value: number) => void;
    setVolume: (value: number) => void;
    togglePlay: () => void;
    toggleMute: () => void;
    toggleLooping: () => void;
    loadAudio: (url: string, name: string) => Promise<void>;
    set: (payload: Partial<PlayerState>) => void;
    reset: () => void;
    setCurrentTime: (value: number) => void;
    repeat: () => void;
    setHandlers: (handlers: AudioHandlers) => void;
  };
}

const DEFAULT_COVER_SVG = `data:image/svg+xml,%3Csvg viewBox='0 0 150 150' xmlns='http://www.w3.org/2000/svg'%3E%3Crect 
x='0' y='0' width='150' height='150' fill='lightgray' /%3E%3Csvg width='50' height='50' viewBox='0 0 24 24' fill='currentColor' 
x='50' y='50'%3E%3Cpath fill='white' fill-rule='evenodd' d='M12 2.25a.75.75 0 0 0-.75.75v11.26a4.25 4.25 0 1 0 1.486 2.888A1 
1 0 0 0 12.75 17V7.75H18a2.75 2.75 0 1 0 0-5.5zm.75 4H18a1.25 1.25 0 1 0 0-2.5h-5.25zm-4.25 8.5a2.75 2.75 0 1 0 0 5.5a2.75 
2.75 0 0 0 0-5.5' clip-rule='evenodd'/%3E%3C/svg%3E%3C/svg%3E`

const DEFAULT_METADATA: AudioMetadata = {
  artist: "Unknown artist",
  cover: DEFAULT_COVER_SVG,
  title: "Unknown title",
};

class AudioManager {
  private audio: HTMLAudioElement | null = null;
  private eventListeners: { event: string; handler: EventListener }[] = [];

  attachEventListener(event: string, handler: EventListener) {
    if (this.audio) {
      this.audio.addEventListener(event, handler);
      this.eventListeners.push({ event, handler });
    }
  }

  removeAllEventListeners() {
    if (this.audio) {
      this.eventListeners.forEach(({ event, handler }) => {
        this.audio?.removeEventListener(event, handler);
      });
      this.eventListeners = [];
    }
  }

  createAudio(): HTMLAudioElement {
    if (this.audio) {
      this.removeAllEventListeners();
      this.audio.pause();
      this.audio.src = "";
      this.audio.load();
    }
    this.audio = new Audio();
    return this.audio;
  }

  getAudio(): HTMLAudioElement | null {
    return this.audio;
  }

  cleanup() {
    if (this.audio) {
      this.removeAllEventListeners();
      this.audio.pause();
      this.audio.src = "";
      this.audio.load();
      this.audio = null;
    }
  }
}

export const useAudioStore = create<PlayerState>((set, get) => {
  const audioManager = new AudioManager();

  const updateMediaSession = (metadata: AudioMetadata) => {
    if ("mediaSession" in navigator && metadata.cover) {
      navigator.mediaSession.metadata = new MediaMetadata({
        artist: metadata.artist,
        artwork: [{ src: metadata.cover, type: "image/jpeg" }],
        title: metadata.title,
      });
    }
  };

  return {
    actions: {
      loadAudio: async (url: string, name: string) => {
        const state = get();

        if (state.metadataController) {
          state.metadataController.abort();
        }

        const controller = new AbortController();
        set({ metadataController: controller });

        try {
          const audio = audioManager.createAudio();

          audioManager.attachEventListener("loadedmetadata", () => {
            set({ duration: audio.duration });
          });

          audioManager.attachEventListener("play", () => {
            set({ isPlaying: true });
          });

          audioManager.attachEventListener("pause", () => {
            set({ isPlaying: false });
          });

          audioManager.attachEventListener("ended", () => {
            get().handlers.nextItem("audio");
            set({ currentTime: 0, isEnded: true });
          });

          const tags = await parseAudioMetadata(url, controller.signal);
          const { artist, title, picture } = tags as Tags;

          const cover = picture ? URL.createObjectURL(picture) : "";
          const metadata = {
            artist: artist || DEFAULT_METADATA.artist,
            cover,
            title: title || name,
          };

          audio.src = url;
          audio.volume = state.volume;
          audio.muted = state.isMuted;
          audio.loop = state.isLooping;
          audio.autoplay = true;
          audio.load();

          updateMediaSession(metadata);

          set({
            audio,
            currentTime: 0,
            error: "",
            isEnded: false,
            isPlaying: true,
            metadata,
          });
        } catch (error) {
          if ((error as Error).name !== "AbortError") {
            set({ error: (error as Error).message });
          }
        }
      },

      repeat: () => {
        const audio = audioManager.getAudio();
        if (audio) {
          audio.currentTime = 0;
          set({ currentTime: 0 });
        }
      },

      reset: () => {
        const state = get();
        if (state.metadataController) {
          state.metadataController.abort();
        }

        audioManager.cleanup();

        set({
          audio: null,
          currentTime: 0,
          duration: 0,
          error: "",
          isEnded: false,
          isLooping: false,
          isMuted: false,
          isPlaying: false,
          metadata: DEFAULT_METADATA,
          metadataController: null,
          volume: 1,
        });
      },

      seek: (value: number) => {
        const audio = audioManager.getAudio();
        if (audio) {
          audio.currentTime = value;
          set({ currentTime: value });
        }
      },

      set: (payload: Partial<PlayerState>) => set(payload),

      setCurrentTime: (value: number) => set({ currentTime: value }),

      setHandlers: (handlers: AudioHandlers) => set({ handlers }),

      setVolume: (value: number) => {
        const audio = audioManager.getAudio();
        if (audio) {
          const normalizedVolume = Math.max(0, Math.min(1, value));
          audio.volume = normalizedVolume;
          set({ volume: normalizedVolume });
        }
      },

      toggleLooping: () => {
        const audio = audioManager.getAudio();
        if (audio) {
          audio.loop = !audio.loop;
          set({ isLooping: audio.loop });
        }
      },

      toggleMute: () => {
        const audio = audioManager.getAudio();
        if (audio) {
          audio.muted = !audio.muted;
          set({ isMuted: audio.muted });
        }
      },

      togglePlay: () => {
        const audio = audioManager.getAudio();
        if (audio) {
          get().isPlaying ? audio.pause() : audio.play();
        }
      },
    },
    audio: null,
    currentTime: 0,
    duration: 0,
    error: "",
    handlers: {
      nextItem: () => {},
      prevItem: () => {},
    },
    isEnded: false,
    isLooping: false,
    isMuted: false,
    isPlaying: false,
    metadata: DEFAULT_METADATA,
    metadataController: null,
    volume: 1,
  };
});

export const audioActions = (state: PlayerState) => state.actions;
