# Multi-Protocol Casting Research Results

Research compiled from web sources to fill gaps in the unified casting implementation plan.

---

## 1. Google Cast Web Sender SDK

### SDK Loading in React SPA

**Official approach**: Load the Cast SDK script dynamically:
```html
<script src="//www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1"></script>
```

**React integration options**:

| Package | Description | Status |
|---------|-------------|--------|
| [react-cast-sender](https://www.npmjs.com/package/react-cast-sender) | Auto-loads SDK, provides hooks | Active |
| [react-redux-chromecast-sender](https://www.npmjs.com/package/react-redux-chromecast-sender) | Redux action integration | Older |
| [react-chromecast](https://github.com/TecladistaProd/react-chromecast) | Hooks-based abstraction | GitHub only |

**Key requirements**:
- HTTPS required (Presentation API deprecated on insecure origins)
- Must handle both Cast button and browser right-click menu casting
- $5 one-time fee to register Chromecast for development mode

### Session Persistence

Sessions are managed through `CastContext`:
```javascript
const context = cast.framework.CastContext.getInstance()
context.setOptions({
  receiverApplicationId: chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID,
  autoJoinPolicy: chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED
})

// Monitor session state
context.addEventListener(
  cast.framework.CastContextEventType.SESSION_STATE_CHANGED,
  (event) => { /* handle state change */ }
)
```

### QueueLoadRequest for Gapless Playback

```javascript
// Create queue items
const queueItems = tracks.map(track =>
  new chrome.cast.media.QueueItem(
    new chrome.cast.media.MediaInfo(track.url, track.mimeType)
  )
)

// Load queue with start index
const request = new chrome.cast.media.QueueLoadRequest(queueItems)
request.startIndex = 0  // Index in array (0-based), not itemId

// Load on session
const session = cast.framework.CastContext.getInstance().getCurrentSession()
await session.loadMedia(request)
```

**Important notes**:
- `startIndex` is array index, not itemId (IDs assigned after queue creation)
- Useful for continuation: user starts locally, casts mid-playback
- Don't modify `queueData` in LOAD interceptor (already applied)

### RemotePlayerController Events

```javascript
const player = new cast.framework.RemotePlayer()
const controller = new cast.framework.RemotePlayerController(player)

controller.addEventListener(
  cast.framework.RemotePlayerEventType.PLAYER_STATE_CHANGED,
  (event) => {
    // player.playerState: IDLE, PLAYING, PAUSED, BUFFERING
    // player.currentTime: updates every second during playback
    // player.duration: total media duration
    // player.isConnected, player.isMediaLoaded, player.isPaused
  }
)
```

**Control methods**: `playOrPause()`, `muteOrUnmute()`, `seek(time)`, `stop()`, `skipAd()`

### Custom vs Default Receiver

| Feature | Default Receiver | Custom Receiver |
|---------|------------------|-----------------|
| Authentication | No token/cookie support | Full control |
| DRM | Not supported | Full Widevine/PlayReady |
| UI Customization | None | Complete |
| Setup complexity | Minimal | Significant |

**Recommendation**: For Navidrome, Default Receiver works since we use token-in-URL auth.

### Error Codes

Common Cast errors to handle:
- `CANCEL`: User cancelled
- `TIMEOUT`: Request timed out
- `API_NOT_INITIALIZED`: SDK not loaded
- `INVALID_PARAMETER`: Bad request data
- `EXTENSION_NOT_COMPATIBLE`: Chrome extension issue
- `RECEIVER_UNAVAILABLE`: No devices found

---

## 2. Apple AirPlay Web Implementation

### WebKit-Specific APIs (Safari Only)

```javascript
const audio = document.querySelector('audio')

// Check availability
audio.addEventListener('webkitplaybacktargetavailabilitychanged', (e) => {
  const available = e.availability === 'available'
  // Show/hide AirPlay button
})

// Show picker (requires user gesture)
airplayButton.onclick = () => {
  audio.webkitShowPlaybackTargetPicker()
}

// Monitor wireless state
audio.addEventListener('webkitcurrentplaybacktargetiswirelesschanged', () => {
  const isWireless = audio.webkitCurrentPlaybackTargetIsWireless
  // Update UI to show AirPlay active
})
```

**User gesture requirement**: `webkitShowPlaybackTargetPicker()` MUST be called from a user-initiated event (click, tap). Cannot be called programmatically.

### Remote Playback API (W3C Standard)

```javascript
const audio = document.querySelector('audio')

if ('remote' in audio) {
  // Watch for device availability
  audio.remote.watchAvailability((available) => {
    // Show/hide cast button
  }).catch(() => {
    // Availability monitoring not supported
  })

  // Prompt device selection (user gesture required)
  audio.remote.prompt().then(() => {
    // Connected
  }).catch((error) => {
    // User cancelled or error
  })

  // Monitor state
  audio.remote.onconnecting = () => { /* connecting */ }
  audio.remote.onconnect = () => { /* connected */ }
  audio.remote.ondisconnect = () => { /* disconnected */ }
}
```

### Browser Support Matrix (2025)

| Browser | AirPlay | Remote Playback API |
|---------|---------|---------------------|
| Safari (macOS/iOS) | WebKit APIs | No |
| Chrome (Android) | No | Yes (Chromecast) |
| Firefox (Android) | No | Limited |
| Chrome (Desktop) | No | No |
| Edge (Windows) | No | Limited (Miracast) |

### AirPlay Behavior

**Key insight**: AirPlay routes the local `<audio>` element to the device. The browser still controls playback. This means:
- No separate driver needed
- LocalDriver + AirPlay routing
- Volume/seek/play/pause all work through local audio element
- No queue support from web (single media only)

**Automatic behavior**: iOS enables system-level AirPlay mirroring, which auto-triggers remote playback of video elements in fullscreen.

### Opt-out Attribute

```html
<!-- Disable AirPlay for specific video -->
<video x-webkit-wirelessvideoplaybackdisabled></video>
```

---

## 3. React Patterns for Media Player Abstraction

### XState for Playback State Machine

From [Sandro Maglione's audio player guide](https://www.sandromaglione.com/articles/getting-started-with-xstate-and-effect-audio-player):

```javascript
import { createMachine } from 'xstate'

const playerMachine = createMachine({
  id: 'player',
  initial: 'idle',
  context: {
    currentTrack: null,
    position: 0,
    volume: 100,
    queue: []
  },
  states: {
    idle: {
      on: { LOAD: 'loading' }
    },
    loading: {
      on: {
        LOADED: 'ready',
        ERROR: 'error'
      }
    },
    ready: {
      on: { PLAY: 'playing' }
    },
    playing: {
      on: {
        PAUSE: 'paused',
        STOP: 'idle',
        ENDED: 'idle'
      }
    },
    paused: {
      on: {
        PLAY: 'playing',
        STOP: 'idle'
      }
    },
    error: {
      on: { RETRY: 'loading' }
    }
  }
})
```

**Benefits**:
- All logic in machine, React components only render based on state
- Team members unfamiliar with XState can still build UI components
- Testable state transitions
- Visual editor for state machine design

### Custom Hook Pattern

From [Stately blog](https://stately.ai/blog/2022-07-18-just-use-hooks-xstate-in-react-components):

```javascript
// usePlayer.js
import { useMachine } from '@xstate/react'
import { playerMachine } from './playerMachine'

export function usePlayer() {
  const [state, send] = useMachine(playerMachine)

  return {
    // State
    isPlaying: state.matches('playing'),
    isPaused: state.matches('paused'),
    currentTrack: state.context.currentTrack,

    // Actions
    play: () => send('PLAY'),
    pause: () => send('PAUSE'),
    loadTrack: (track) => send({ type: 'LOAD', track })
  }
}
```

### useActorRef for Minimal Re-renders

```javascript
import { useActorRef } from '@xstate/react'

// Returns static reference, doesn't re-render on state changes
const actorRef = useActorRef(playerMachine)

// Selectively subscribe to specific state
const isPlaying = useSelector(actorRef, (state) => state.matches('playing'))
```

### Context vs Redux

| Approach | Pros | Cons |
|----------|------|------|
| Context + XState | Simpler, built-in state machine | Prop drilling for large apps |
| Redux + XState | Global state, time-travel debugging | More boilerplate |
| Context + useReducer | No extra deps | Manual state machine logic |

**Recommendation**: Context + XState for media player (isolated concern, clear state machine)

---

## 4. Gapless Playback Technical Details

### HTML5 Audio Double-Buffering

From [Gapless-5](https://github.com/regosen/Gapless-5):

```javascript
// Pool of 3 audio elements: previous, current, next
const audioPool = [
  new Audio(),
  new Audio(),
  new Audio()
]

let currentIndex = 1

function preloadNext(url) {
  const nextIndex = (currentIndex + 1) % 3
  audioPool[nextIndex].src = url
  audioPool[nextIndex].preload = 'auto'
}

function playNext() {
  audioPool[currentIndex].pause()
  currentIndex = (currentIndex + 1) % 3
  audioPool[currentIndex].play()
  preloadNext(getNextTrackUrl())
}
```

**Key libraries**:
- [Gapless-5](https://github.com/regosen/Gapless-5) - HTML5 + WebAudio hybrid
- [gapless.js](https://github.com/RelistenNet/gapless.js) - Web Audio API based
- [Stitches](https://github.com/sudara/stitchES) - ES6 double-buffering

### Challenges

1. **AudioBuffer duration inaccuracy** in many browsers
2. **setTimeout insufficient** - millisecond resolution can't prevent gaps
3. **WebAudio memory issue** - must download entire track before playback
4. **MP3 encoder padding** - adds silence at start/end of MP3 files

### Gapless-5 Hybrid Approach

```javascript
// Start with HTML5 Audio while WebAudio loads
// Switch seamlessly to WebAudio once decoded
// Optional: 25-50ms crossfade between tracks
```

### Chromecast Queue Pre-buffering

```javascript
const queueItem = new chrome.cast.media.QueueItem(mediaInfo)
queueItem.preloadTime = 10 // Preload 10 seconds before end of current track
```

---

## 5. Authentication for Casting

### Token-in-URL Approach

```javascript
// Current Navidrome approach (Subsonic API)
const token = md5(password + salt)
const url = `${baseUrl}/rest/stream?id=${trackId}&u=${user}&t=${token}&s=${salt}`
```

**Security considerations**:
- Tokens visible in browser history, server logs, referrer headers
- Use short-lived tokens (expire after session/1 hour)
- Consider one-time-use tokens for sensitive content

### HTTPS Requirements

| Protocol | HTTPS Required? |
|----------|-----------------|
| Chromecast | Yes (Presentation API) |
| AirPlay | No (but recommended) |
| Sonos | No (LAN only) |

**Chromecast gotcha**: Chromecast uses Google DNS (8.8.8.8, 8.8.4.4), so self-signed certs and local DNS won't work. Need valid public SSL cert.

### DRM (If Needed Later)

For DRM-protected content on Chromecast:
- Requires Custom Receiver (not Default)
- Must implement license URL and token handling
- Widevine/PlayReady support available

---

## 6. Real-World Examples

### Jellyfin Chromecast Implementation

[jellyfin-chromecast](https://github.com/jellyfin/jellyfin-chromecast) - Official Cast Web Receiver

**Key insights**:
- Challenge: Chromecast requires HTTPS with valid cert
- Challenge: Chromecast uses Google DNS (local domains don't resolve)
- Solution: Public domain + reverse proxy, or use Jellyfin MPV Shim

### Jellyfin MPV Shim

[jellyfin-mpv-shim](https://github.com/jellyfin/jellyfin-mpv-shim) - Open source Chromecast alternative

- Based on Plex MPV Shim
- Direct play (no transcoding)
- Full subtitle support
- MIT License

### Streamyfin (React Native)

[streamyfin](https://github.com/streamyfin/streamyfin) - Modern Jellyfin client with Expo

- Chromecast support in development
- React Native + Expo
- MPL-2.0 license

### NPM Packages Summary

| Package | Platform | Type |
|---------|----------|------|
| react-cast-sender | Web | Chromecast hooks |
| react-native-google-cast | React Native | Native SDK wrapper |
| gapless.js | Web | Gapless playback |
| Gapless-5 | Web | Gapless playback |

---

## Implementation Recommendations

### Phase 1: Player Abstraction
1. Use XState for playback state machine
2. Create `usePlayer` hook with driver abstraction
3. Start with LocalDriver wrapping current player

### Phase 2: Sonos Enhancement
1. Add SetNextAVTransportURI for gapless
2. Implement WebSocket for real-time state sync

### Phase 3: Chromecast
1. Use [react-cast-sender](https://www.npmjs.com/package/react-cast-sender) for SDK loading
2. Default Receiver (token-in-URL works)
3. Add Content-Range/Accept-Ranges CORS headers

### Phase 4: AirPlay
1. Detect Safari with `'webkitShowPlaybackTargetPicker' in audio`
2. Use existing LocalDriver + WebKit event handlers
3. No queue support (single track only)

---

## Sources

### Google Cast SDK
- [Integrate Cast SDK](https://developers.google.com/cast/docs/web_sender/integrate)
- [QueueLoadRequest](https://developers.google.com/cast/docs/reference/web_sender/chrome.cast.media.QueueLoadRequest)
- [RemotePlayerController](https://developers.google.com/cast/docs/reference/web_sender/cast.framework.RemotePlayerController)
- [Queueing](https://developers.google.com/cast/docs/web_receiver/queueing)

### AirPlay
- [Remote Playback API - MDN](https://developer.mozilla.org/en-US/docs/Web/API/Remote_Playback_API)
- [webkitShowPlaybackTargetPicker - Apple](https://developer.apple.com/documentation/webkitjs/htmlmediaelement/1632172-webkitshowplaybacktargetpicker)
- [Adding AirPlay Button - Apple](https://developer.apple.com/documentation/webkitjs/adding_an_airplay_button_to_your_safari_media_controls)

### React Patterns
- [XState + Effect Audio Player](https://www.sandromaglione.com/articles/getting-started-with-xstate-and-effect-audio-player)
- [React Video Player with XState](https://joelhooks.com/react-video-player)
- [@xstate/react](https://stately.ai/docs/xstate-react)
- [Just Use Hooks - Stately](https://stately.ai/blog/2022-07-18-just-use-hooks-xstate-in-react-components)

### Gapless Playback
- [Gapless-5](https://github.com/regosen/Gapless-5)
- [gapless.js](https://github.com/RelistenNet/gapless.js)
- [Stitches](https://github.com/sudara/stitchES)

### Open Source Examples
- [jellyfin-chromecast](https://github.com/jellyfin/jellyfin-chromecast)
- [jellyfin-mpv-shim](https://github.com/jellyfin/jellyfin-mpv-shim)
- [streamyfin](https://github.com/streamyfin/streamyfin)
- [react-cast-sender](https://www.npmjs.com/package/react-cast-sender)
