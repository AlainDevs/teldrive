import { memo, useRef } from "react";
import type Artplayer from "artplayer";
import type { Option } from "artplayer";
import { Player } from "./art-player";

interface VideoPlayerProps {
  url: string;
}
const VideoPlayer = memo(({ url, ...props }: VideoPlayerProps) => {
  const artInstance = useRef<Artplayer | null>(null);
  const artOptions: Option = {
    airplay: true,
    aspectRatio: true,
    autoMini: true,
    autoOrientation: true,
    autoPlayback: true,
    autoSize: false,
    autoplay: true,
    backdrop: true,
    container: "",
    fastForward: true,
    flip: true,
    fullscreen: true,
    fullscreenWeb: true,
    hotkey: true,
    lock: true,
    moreVideoAttr: {
      playsInline: true,
    },
    muted: false,
    mutex: true,
    pip: true,
    playbackRate: true,
    playsInline: true,
    screenshot: true,
    setting: true,
    url,
    volume: 0.6,
  };

  return (
    <Player
      style={{ aspectRatio: "16 /9" }}
      ref={artInstance}
      option={artOptions}
      {...props}
    />
  );
});

export default memo(VideoPlayer);
