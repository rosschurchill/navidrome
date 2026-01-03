# Research Request: Multi-Protocol Music Casting Implementation

## 1. Google Cast Web Sender SDK
- SDK loading best practices in React SPA
- Session persistence across page refreshes
- QueueLoadRequest for gapless playback (detailed API)
- RemotePlayerController event types and frequency
- Error codes and recovery strategies
- Custom vs Default Receiver trade-offs

## 2. Apple AirPlay Web Implementation
- webkitShowPlaybackTargetPicker() user gesture requirements
- Remote Playback API (W3C) browser support matrix 2024/2025
- URL handoff vs browser streaming - when does each occur?
- Volume control capabilities from web
- Queue/gapless support limitations

## 3. React Patterns for Media Player Abstraction
- State machine (XState) for playback states?
- Hook composition for multiple device types
- Context vs Redux for real-time media state
- Audio element abstraction patterns

## 4. Gapless Playback Technical Details
- HTML5 Audio double-buffering implementation
- Chromecast QueueItem pre-buffering config
- MP3 encoder padding handling

## 5. Authentication for Casting
- Token-in-URL security best practices
- Token expiration for long sessions
- HTTPS requirements per protocol

## 6. Real-World Examples
- Open source Chromecast React implementations
- Plex/Jellyfin web casting code
- npm packages for multi-protocol casting

For each area provide: code examples, gotchas, browser compatibility, and documentation links.
