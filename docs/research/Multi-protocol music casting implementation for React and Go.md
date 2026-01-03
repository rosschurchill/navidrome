# Multi-protocol music casting implementation for React and Go

Chromecast and AirPlay represent fundamentally different integration paths for web-based music players. **Chromecast offers full queue management and SDK-driven control**, while **AirPlay from browsers is severely limited**—no queues, no gapless playback, and no programmatic volume control. This report provides implementation-ready technical details for building casting into a Navidrome-style music server with React frontend and Go backend.

## Google Cast SDK loading and React integration

The Cast SDK must load via a specific callback mechanism that requires careful handling in React's lifecycle. The SDK script triggers `window.__onGCastApiAvailable` when ready, creating a race condition if the callback isn't set before script loading.

```typescript
import { useEffect, useState, useCallback, useRef } from 'react';

interface CastState {
  isAvailable: boolean;
  isConnected: boolean;
  castSession: cast.framework.CastSession | null;
}

export function useCastSDK(applicationId: string) {
  const [castState, setCastState] = useState<CastState>({
    isAvailable: false,
    isConnected: false,
    castSession: null,
  });
  const initializationRef = useRef(false);

  const initializeCastApi = useCallback(() => {
    if (initializationRef.current) return;
    initializationRef.current = true;

    const context = cast.framework.CastContext.getInstance();
    context.setOptions({
      receiverApplicationId: applicationId,
      autoJoinPolicy: chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED,
    });

    setCastState(prev => ({ ...prev, isAvailable: true }));
  }, [applicationId]);

  useEffect(() => {
    if (window.chrome?.cast && window.cast?.framework) {
      initializeCastApi();
      return;
    }

    // Set callback BEFORE loading script to avoid race condition
    window.__onGCastApiAvailable = (isAvailable: boolean) => {
      if (isAvailable) initializeCastApi();
    };

    const existingScript = document.querySelector('script[src*="cast_sender.js"]');
    if (!existingScript) {
      const script = document.createElement('script');
      script.src = 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1';
      script.async = true;
      document.body.appendChild(script);
    }
  }, [initializeCastApi]);

  return castState;
}
```

**Key gotchas**: React Strict Mode causes double initialization—use a ref guard. The SDK may load from cache before component mounts, so check for existing SDK availability. Never remove `__onGCastApiAvailable` on cleanup as the callback may still fire.

## Session persistence works automatically with proper AutoJoinPolicy

The Cast SDK handles session restoration internally based on the `AutoJoinPolicy` setting. `ORIGIN_SCOPED` reconnects across tabs with the same origin; `TAB_AND_ORIGIN_SCOPED` restricts to the same tab. Listen for `SESSION_RESUMED` to detect reconnection after page refresh:

```typescript
const context = cast.framework.CastContext.getInstance();

context.addEventListener(
  cast.framework.CastContextEventType.SESSION_STATE_CHANGED,
  (event) => {
    switch (event.sessionState) {
      case cast.framework.SessionState.SESSION_RESUMED:
        // Reconnected to active cast session after refresh
        const session = context.getCurrentSession();
        syncUIWithCastState(session);
        break;
      case cast.framework.SessionState.SESSION_ENDED:
        cleanupCastUI();
        break;
    }
  }
);
```

Session data persists on the Chromecast device itself, not in browser storage. The SDK automatically attempts reconnection when the app returns to foreground or page reloads.

## Chromecast cannot achieve true gapless playback

**Critical limitation**: Chromecast does not natively support gapless playback. The receiver plays individual tracks sequentially with small gaps. Apps claiming gapless streaming achieve this by transcoding on the sender side into a single continuous audio stream.

The `QueueLoadRequest` API provides queue management with pre-buffering, which minimizes but doesn't eliminate gaps:

```typescript
const createQueueItem = (track: Track): chrome.cast.media.QueueItem => {
  const mediaInfo = new chrome.cast.media.MediaInfo(track.contentUrl, 'audio/mpeg');
  
  const metadata = new chrome.cast.media.MusicTrackMediaMetadata();
  metadata.title = track.title;
  metadata.artist = track.artist;
  metadata.albumName = track.album;
  metadata.images = [new chrome.cast.Image(track.coverArtUrl)];
  
  mediaInfo.metadata = metadata;
  mediaInfo.streamType = chrome.cast.media.StreamType.BUFFERED;
  
  const queueItem = new chrome.cast.media.QueueItem(mediaInfo);
  queueItem.autoplay = true;
  queueItem.preloadTime = 20; // Start loading 20s before current track ends
  return queueItem;
};

const loadQueue = async (session: cast.framework.CastSession, tracks: Track[]) => {
  const queueItems = tracks.map(createQueueItem);
  const queueLoadRequest = new chrome.cast.media.QueueLoadRequest(queueItems);
  queueLoadRequest.repeatMode = chrome.cast.media.RepeatMode.ALL;
  
  await new Promise((resolve, reject) => {
    session.getSessionObj().queueLoad(queueLoadRequest, resolve, reject);
  });
};
```

**preloadTime** defaults to **20 seconds**; values between **15-30 seconds** are optimal for audio. Queue management methods include `queueInsertItems`, `queueRemoveItems`, `queueReorderItems`, `queueNext`, `queuePrev`, and `queueJumpToItem`.

## RemotePlayerController events require throttling for performance

The `CURRENT_TIME_CHANGED` event fires approximately once per second during playback—frequent enough to cause performance issues if directly bound to React state.

| Event | Frequency | Use Case |
|-------|-----------|----------|
| `CURRENT_TIME_CHANGED` | ~1/sec during playback | Progress bar, seek preview |
| `PLAYER_STATE_CHANGED` | On state transitions | Play/pause button state |
| `MEDIA_INFO_CHANGED` | When track changes | Update now-playing UI |
| `VOLUME_LEVEL_CHANGED` | On volume adjustment | Volume slider sync |
| `ANY_CHANGE` | Every update | Avoid—highest frequency |

```typescript
import { throttle, debounce } from 'lodash';

const controller = new cast.framework.RemotePlayerController(player);

// Throttle time updates to max 4/second
const handleTimeChange = throttle(() => {
  setCurrentTime(player.currentTime);
}, 250);

// Debounce state changes for rapid transitions
const handleStateChange = debounce(() => {
  setPlayerState(player.playerState);
}, 100);

controller.addEventListener(
  cast.framework.RemotePlayerEventType.CURRENT_TIME_CHANGED,
  handleTimeChange
);
```

## Cast SDK error codes and recovery strategies

| Error Code | Description | Recovery |
|------------|-------------|----------|
| `TIMEOUT` | Operation timed out | Exponential backoff retry |
| `SESSION_ERROR` | Session creation failed | Request new session |
| `LOAD_MEDIA_FAILED` | Media load failed | Verify URL/CORS, retry |
| `RECEIVER_UNAVAILABLE` | No receivers found | Show device discovery UI |
| `EXTENSION_MISSING` | Cast extension missing | Prompt extension install |
| `CHANNEL_ERROR` | Communication failed | Re-establish connection |

Receiver-side errors follow a composite format: `{ErrorCode}{NetworkErrorCode}{HTTPStatus}`. For example, `3016404` means segment fetch failed with HTTP 404.

## Custom receiver required for advanced scenarios

Use the **Default Media Receiver** (`chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID`) for prototyping and basic playback. A **Custom Receiver** requires:

1. Registration at https://cast.google.com/publish (**$5 one-time fee**)
2. Self-hosted receiver HTML on HTTPS
3. Application ID for sender configuration

Custom receivers enable: branded UI, DRM (Widevine), custom authentication flows, advanced queue logic, and server-side analytics. For a Navidrome-style server, the Default Receiver works for MVP; consider custom for branding and token-based auth handling.

## AirPlay from web has fundamental limitations

Safari uses webkit-prefixed APIs exclusively—the W3C Remote Playback API does **not** support AirPlay (it only works for Chromecast in Chrome). **Critical constraints**:

- **No queue support**: Each track is a separate session
- **No gapless playback**: 1-2 second gaps between tracks unavoidable
- **No volume control**: `HTMLMediaElement.volume` doesn't affect receiver volume
- **User gesture required**: `webkitShowPlaybackTargetPicker()` must be called within ~5 seconds of user interaction

```typescript
function AirPlayButton({ audioRef }: { audioRef: RefObject<HTMLAudioElement> }) {
  const [available, setAvailable] = useState(false);
  const [wireless, setWireless] = useState(false);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || typeof audio.webkitShowPlaybackTargetPicker !== 'function') return;

    const handleAvailability = (e: any) => {
      setAvailable(e.availability === 'available');
    };

    const handleWirelessChange = () => {
      setWireless(audio.webkitCurrentPlaybackTargetIsWireless);
    };

    audio.addEventListener('webkitplaybacktargetavailabilitychanged', handleAvailability);
    audio.addEventListener('webkitcurrentplaybacktargetiswirelesschanged', handleWirelessChange);

    return () => {
      audio.removeEventListener('webkitplaybacktargetavailabilitychanged', handleAvailability);
      audio.removeEventListener('webkitcurrentplaybacktargetiswirelesschanged', handleWirelessChange);
    };
  }, [audioRef]);

  // MUST be called directly from click handler
  const showPicker = () => audioRef.current?.webkitShowPlaybackTargetPicker();

  if (!available) return null;
  return <button onClick={showPicker}>{wireless ? 'AirPlay Active' : 'AirPlay'}</button>;
}
```

## URL handoff versus browser streaming for AirPlay

When AirPlay activates, Safari decides between two modes:

**URL Handoff** (receiver fetches directly): Used for direct URLs (MP4, HLS m3u8). Receiver makes its own HTTP request—no browser cookies or headers are sent. Authentication must be embedded in URL.

**Media Remoting** (browser streams): Used for Media Source Extensions (MSE), blob URLs, or authenticated content. Browser decodes and streams frames to receiver. More bandwidth through source device.

For MSE-based players, Safari 16.4+ supports automatic fallback. Provide an HLS source element that Safari switches to when AirPlay activates:

```html
<video>
  <source type="video/mp4" src="blob:..." /> <!-- MSE for local -->
  <source type="application/x-mpegURL" src="https://server.com/stream.m3u8" /> <!-- HLS for AirPlay -->
</video>
```

## Zustand outperforms Context API for real-time media state

React Context causes full subtree re-renders on any state change—problematic for `currentTime` updating 4x/second. Zustand with selectors provides granular subscriptions:

```typescript
import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';

interface MediaState {
  currentTime: number;
  duration: number;
  isPlaying: boolean;
  queue: Track[];
  setCurrentTime: (time: number) => void;
}

export const useMediaStore = create<MediaState>()(
  subscribeWithSelector((set) => ({
    currentTime: 0,
    duration: 0,
    isPlaying: false,
    queue: [],
    setCurrentTime: (time) => set({ currentTime: time }),
  }))
);

// Components subscribe only to needed slices
function ProgressBar() {
  const currentTime = useMediaStore((s) => s.currentTime);
  const duration = useMediaStore((s) => s.duration);
  // Only re-renders when these values change
  return <Slider value={currentTime / duration} />;
}

function QueueList() {
  const queue = useMediaStore((s) => s.queue);
  // Never re-renders from currentTime updates
  return <ul>{queue.map(t => <li key={t.id}>{t.title}</li>)}</ul>;
}
```

**Architectural recommendation**: Separate stores for ephemeral state (currentTime, buffered) versus persistent state (queue, volume). This prevents high-frequency updates from invalidating low-frequency state.

## PlaybackEngine abstraction enables multi-device support

Abstract `HTMLAudioElement` and Cast SDK behind a common interface. This enables device switching without rewriting UI logic:

```typescript
interface PlaybackEngine {
  play(): Promise<void>;
  pause(): void;
  seek(time: number): void;
  setVolume(volume: number): void;
  loadSource(url: string, metadata?: MediaMetadata): Promise<void>;
  getCurrentTime(): number;
  getDuration(): number;
  on<K extends keyof PlaybackEvents>(event: K, handler: PlaybackEvents[K]): void;
  destroy(): void;
}

// Factory creates appropriate engine
function createPlaybackEngine(type: 'local' | 'chromecast'): PlaybackEngine {
  switch (type) {
    case 'chromecast': return new CastPlaybackEngine();
    default: return new LocalPlaybackEngine();
  }
}
```

**AirPlay note**: AirPlay doesn't need a separate engine—it uses the same `HTMLAudioElement` with the Remote Playback API handling the routing transparently.

## HTML5 gapless playback requires hybrid Audio + WebAudio approach

HTML5 Audio cuts off the last chunk of audio; WebAudio requires full file download before playback. The solution combines both:

```typescript
class GaplessPlayer {
  private audioContext = new AudioContext();
  private html5Audio = new Audio();
  private nextBuffer: AudioBuffer | null = null;

  async loadTrack(url: string): Promise<void> {
    // HTML5 for immediate streaming playback
    this.html5Audio.src = url;
    this.html5Audio.preload = 'auto';
    
    // WebAudio for gapless transition (requires full download)
    const response = await fetch(url);
    const arrayBuffer = await response.arrayBuffer();
    this.nextBuffer = await this.audioContext.decodeAudioData(arrayBuffer);
  }

  // Switch to WebAudio before track end for sample-accurate transition
  scheduleGaplessTransition(nextTrackBuffer: AudioBuffer, startTime: number): void {
    const source = this.audioContext.createBufferSource();
    source.buffer = nextTrackBuffer;
    source.connect(this.audioContext.destination);
    source.start(startTime);
  }
}
```

**MSE approach** (more robust): Use Media Source Extensions with `appendWindowStart`/`appendWindowEnd` to trim encoder padding. Parse gapless metadata from iTunSMPB tags or LAME headers to determine exact trim points.

## MP3 encoder padding requires parsing metadata for trimming

LAME encoder adds ~576 samples at start and end. Gapless metadata is embedded in iTunSMPB (Apple) or LAME/Xing headers:

```typescript
function parseGaplessMetadata(arrayBuffer: ArrayBuffer): { frontPadding: number; endPadding: number } {
  const byteStr = new TextDecoder().decode(arrayBuffer.slice(0, 512));
  const SAMPLE_RATE = 44100;
  
  // Parse iTunSMPB format: "0000000 00000840 000001C0 0000000000046E00"
  const iTunesIndex = byteStr.indexOf('iTunSMPB');
  if (iTunesIndex !== -1) {
    const frontPadding = parseInt(byteStr.substr(iTunesIndex + 34, 8), 16);
    const endPadding = parseInt(byteStr.substr(iTunesIndex + 43, 8), 16);
    return { frontPadding, endPadding };
  }
  
  // Parse LAME header (look for "LAME" or "Lavf" tag)
  const lameIndex = byteStr.indexOf('LAME');
  if (lameIndex !== -1) {
    const gaplessBits = /* read 3 bytes at lameIndex + 21 */;
    return { frontPadding: gaplessBits >> 12, endPadding: gaplessBits & 0xFFF };
  }
  
  return { frontPadding: 0, endPadding: 0 };
}
```

**Recommendation**: Use FLAC or Opus for gapless playback. Both formats define precise sample boundaries without encoder padding issues.

## Token-in-URL authentication required for Chromecast

Chromecast's Default Media Receiver cannot send custom HTTP headers. Authentication must be embedded in the URL itself. Use HMAC-signed URLs with short expiration:

```go
package auth

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "fmt"
    "net/url"
    "time"
)

func GenerateSignedStreamURL(baseURL, trackID, userID string, secret []byte, ttl time.Duration) string {
    exp := time.Now().Add(ttl).Unix()
    
    // Build message for signing
    message := fmt.Sprintf("%s|%s|%d", trackID, userID, exp)
    
    h := hmac.New(sha256.New, secret)
    h.Write([]byte(message))
    signature := base64.URLEncoding.EncodeToString(h.Sum(nil))
    
    params := url.Values{}
    params.Set("track", trackID)
    params.Set("user", userID)
    params.Set("exp", fmt.Sprintf("%d", exp))
    params.Set("sig", signature)
    
    return baseURL + "?" + params.Encode()
}
```

**Token lifetime strategy**: Use 1-2 hour tokens for individual tracks. For long listening sessions with queues, implement proactive token refresh—update queue items with fresh URLs before expiration.

## HTTPS with CA-signed certificates mandatory for Chromecast

Chromecast **strictly requires HTTPS with certificates signed by a public CA**. Self-signed certificates do not work. For local network servers (192.168.x.x):

1. **Use a real domain pointing to private IP**: Set DNS A record for `music.yourdomain.com` → `192.168.1.100`, then get Let's Encrypt certificate for the domain
2. **Reverse proxy**: nginx with valid certificate proxying to local app
3. **Development**: Use ngrok or Cloudflare Tunnel for HTTPS wrapper

AirPlay is more flexible—doesn't strictly require HTTPS for local streaming, uses its own AES encryption.

## Jellyfin provides the most mature open-source casting reference

**Jellyfin** has production-ready Chromecast implementation:
- Web client sender: https://github.com/jellyfin/jellyfin-web (`src/plugins/chromecastPlayer/`)
- Custom CAF receiver: https://github.com/jellyfin/jellyfin-chromecast

Key architectural patterns from Jellyfin:
- Plugin-based sender architecture for clean separation
- TypeScript throughout
- JWT tokens for authenticated stream URLs
- Server-side transcoding decision logic

**npm packages** worth considering:
- **Cast.js** (https://github.com/castjs/castjs): Vanilla JS with clean event-based API, better maintained than React-specific packages
- **react-cast-sender**: Provides `useCast` and `useCastPlayer` hooks but hasn't been updated in 5 years

For AirPlay, no viable web libraries exist—mDNS discovery and RTSP protocol aren't available in browsers. Consider **AirConnect** (https://github.com/philippe44/AirConnect) as a server-side bridge that makes Chromecast devices appear as AirPlay targets.

## Conclusion

Building multi-protocol casting into a Navidrome-style player requires accepting significant asymmetry between Chromecast and AirPlay capabilities. **Chromecast offers full programmatic control** with queue management, session persistence, and volume control—implement it first as the primary casting target. **AirPlay from web is fundamentally limited** to single-track playback with gaps; it's best treated as a progressive enhancement for Safari users rather than a first-class feature.

For the React architecture, use **Zustand for state management** with separate stores for ephemeral and persistent state. Implement a **PlaybackEngine abstraction** to decouple UI from playback implementation. For authentication, **HMAC-signed URLs with 1-2 hour expiration** work well for cast sessions; implement proactive token refresh for queue items. Study **Jellyfin's implementation** as the most production-ready open-source reference—their plugin architecture and receiver code demonstrate patterns that have been battle-tested at scale.