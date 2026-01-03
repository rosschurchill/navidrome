# **Modernizing Navidrome: A Technical Implementation Report on Chromecast and AirPlay Integration**

## **1\. Executive Summary**

The modernization of the Navidrome music server repository represents a pivotal transition from a locally-consumed web application to a distributed media ecosystem. While the current architecture robustly supports browser-based playback and Subsonic-compatible mobile clients, the contemporary user expectation has shifted towards a "Bring Your Own Screen" (BYOS) paradigm. In this model, the web interface serves not merely as a playback endpoint, but as a sophisticated remote control for high-fidelity listening hardware. The user's current initiative to implement Sonos casting is a foundational step; however, achieving comprehensive market parity requires the integration of the two dominant wireless streaming protocols: Google Cast (Chromecast) and Apple AirPlay.  
This report provides an exhaustive technical analysis and implementation roadmap for integrating Chromecast and AirPlay into the Navidrome ecosystem. Unlike Sonos, which often operates on UPnP-derived or proprietary TCP architectures manageable via backend discovery, Chromecast and AirPlay introduce distinct challenges that bridge the frontend browser environment and the backend streaming infrastructure. The implementation of Chromecast requires a rigid adherence to the Google Cast SDK's Sender/Receiver model, necessitating complex state synchronization between the React frontend and the remote device. Conversely, AirPlay integration is heavily dependent on browser-native APIs within the WebKit ecosystem, requiring a tiered detection strategy to ensure cross-platform stability.  
Crucially, this research identifies that the primary friction points for this implementation will not be in the control logic, but in the backend delivery mechanisms. Specifically, the strict Cross-Origin Resource Sharing (CORS) policies enforced by Chromecast receivers, the handling of authentication tokens in stream URLs, and the negotiation of audio codecs (specifically ALAC and FLAC) for AirPlay endpoints require targeted engineering efforts within the Go backend. This document delineates the necessary architectural modifications, utilizing the existing go-chi routing middleware and ffmpeg transcoding pipelines, to streamline this implementation. By synthesizing data from current GitHub issues, developer documentation, and community analyses, this report serves as a definitive guide for engineering a robust, multi-protocol streaming experience for Navidrome.

## ---

**2\. Architectural Analysis of the Existing Navidrome Ecosystem**

To effectively overlay casting protocols onto Navidrome, one must first deconstruct the existing architectural constraints and capabilities. The "old" Navidrome repo serves as a solid foundation, but its design was primarily optimized for a client-pull model where the browser requesting the page is identical to the browser playing the audio. Remote casting breaks this assumption.

### **2.1 Backend Architecture (Go & Chi)**

Navidrome's backend is constructed using the Go programming language, utilizing the go-chi router for HTTP request handling.1 This lightweight, idiomatic router provides the necessary middleware hooks to manage the complex request lifecycles associated with media streaming.

#### **2.1.1 The Subsonic API Surface**

The core of Navidrome's interaction model is its compatibility with the Subsonic API (currently v1.16.1).3 This is a strategic advantage for casting. The casting protocols do not require a proprietary API; they simply need a valid URL pointing to a media stream. The Subsonic stream endpoint (/rest/stream) is the critical interface here. When a user initiates a cast, the React frontend effectively hands off a constructed stream URL to the remote device.  
However, the existing implementation of this endpoint is optimized for direct client consumption. The id parameter used in these calls (e.g., id=12345) references a media entity in the database. The system supports transcoding via ffmpeg, controlled by parameters such as format and maxBitRate.4 For casting, these parameters become rigid constraints. A Chromecast Audio device may support FLAC natively, but a first-generation Chromecast video dongle plugged into a receiver may struggle with high-bitrate audio decoding without stuttering. The backend must therefore be robust enough to handle "transcode-on-demand" requests initiated not by the user's settings in the web UI, but by the capability negotiation logic of the casting subsystem.

#### **2.1.2 Middleware and Routing Constraints**

The go-chi router utilizes a stack of middlewares for logging, recovery, and security headers.6 Detailed analysis of the codebase suggests that the current CORS configuration might be too restrictive for remote playback devices. When a browser plays media, the Origin is the same as the server. When a Chromecast plays media, the Origin is often chrome-extension://\[cast-extension-id\] or a generic Google receiver URL. If the go-chi middleware detects a mismatch and blocks the request or fails to send the Access-Control-Allow-Origin header, playback will fail silently on the TV while the web UI appears functional. This "split-brain" state is a common failure mode in casting implementations and requires specific middleware adjustments in server/server.go or equivalent entry points.

### **2.2 Frontend Architecture (React & Material UI)**

The frontend is a Single Page Application (SPA) built with React and Material UI.8 The current player component structure, likely found within ui/src/components/Player, is designed around a local audio element—either a standard HTML5 \<audio\> tag or a wrapper library like react-music-player or react-h5-audio-player.10

#### **2.2.1 State Management Bottlenecks**

The existing state management relies on React's internal state or perhaps a Context API provider that tracks properties like isPlaying, currentTime, and volume based on events fired by the local DOM element. This tight coupling is the primary obstacle to casting.

* **Current Flow:** User clicks Play \-\> React updates State \-\> DOM \<audio\> src is set \-\> Audio plays \-\> onTimeUpdate event fires \-\> React updates Seek Bar.  
* **Required Flow (Casting):** User clicks Play \-\> React checks Active Output Device.  
  * *If Local:* Execute Current Flow.  
  * *If Cast:* Send LOAD message to Cast SDK \-\> Wait for Cast SDK PLAYER\_STATE\_CHANGED event \-\> Update Seek Bar.

This necessitates a "State Lift." The source of truth for playback status can no longer be the DOM element; it must be an abstract PlayerEngine that subscribes to whichever driver (Local, Cast, AirPlay, Sonos) is currently active. The research indicates that Navidrome's frontend is moving towards functional components and Hooks 12, which is the ideal pattern for implementing this abstraction. Hooks like useCastSession and useAirPlayAvailability can encapsulate the complex event listeners required by these protocols, exposing a unified API to the UI components.

#### **2.2.2 Component Hierarchy and "Jukebox" Distinction**

It is critical to distinguish "Casting" from Navidrome's existing "Jukebox Mode".8 Jukebox mode instructs the *server hardware* to play audio (e.g., via a sound card attached to the Raspberry Pi running Navidrome). Casting instructs a *remote client* to fetch audio. These are functionally opposite data flows. The React UI must cleanly separate these concepts to avoid user confusion. The "Connect" menu design must delineate between "Server Output" (Jukebox) and "Network Speakers" (Cast/AirPlay/Sonos).

## ---

**3\. Google Cast (Chromecast): Protocol and Implementation**

Google Cast is a proprietary protocol that allows a "Sender" application to initiate and control media playback on a "Receiver" device. For a web-based Navidrome client, this interaction is governed by the Google Cast Web Sender SDK (Chrome Sender Framework).

### **3.1 The Cast SDK Ecosystem**

The Cast architecture is tripartite: the Sender (Navidrome Web UI), the Receiver (Chromecast Device), and the Google Cast Service (Discovery/Signaling).

#### **3.1.1 The Web Sender Framework**

The Sender SDK is the library that runs inside the Chrome browser. It is not available as a standard NPM package and must be loaded dynamically from Google's CDN (www.gstatic.com/cv/js/sender/v1/cast\_sender.js?loadCastFramework=1).15 This introduces an asynchronous dependency into the React application. The React app cannot render the Cast button until this script is loaded and the global cast object is initialized.  
To manage this in a React environment, a robust loading utility is required. The utility must define a global callback window.\_\_onGCastApiAvailable. This callback is invoked by the Google script once it is ready.

* **Initialization Logic:** Inside this callback, the application must initialize the CastContext via cast.framework.CastContext.getInstance().setOptions().  
* **Receiver Application ID:** For basic audio streaming, the chrome.cast.media.DEFAULT\_MEDIA\_RECEIVER\_APP\_ID is sufficient and requires no registration.16 However, strictly using the default receiver limits UI customization on the TV screen (e.g., branding, colors). If the Navidrome team wishes to display the Navidrome logo and a custom background on the TV, a "Styled Media Receiver" must be registered in the Google Cast Developer Console, providing a CSS URL to style the default player.17

### **3.2 Sender Application Engineering**

The integration into the React frontend involves creating a bridge between the imperative Cast SDK and the declarative React component lifecycle.

#### **3.2.1 The useCastSender Hook**

Rather than scattering Cast logic throughout the App.js or Player.js, the implementation should utilize a custom hook, useCastSender.16 This hook serves as the interface for the rest of the application.

* **Availability Detection:** The hook subscribes to the cast.framework.CastContextEventType.CAST\_STATE\_CHANGED event. When the state changes to NO\_DEVICES\_AVAILABLE, the hook should return false for availability, triggering the UI to hide the Cast button. This prevents UI clutter for users on networks without Cast devices.19  
* **Session Management:** The hook wraps CastContext.getInstance().requestSession(). This function opens the native Chrome Cast dialog. Upon successful connection, the hook must store the active CastSession object in a React Context or Global Store (like Redux, though Context is preferred for this scoped state).20

#### **3.2.2 Constructing the Load Request**

When the user selects a track, the frontend must construct a LoadRequest object instead of setting an \<audio\> src. This object maps the Navidrome song metadata to the Google Cast media schema.

* **contentId:** This is the *most critical* field. It must be the absolute URL to the stream endpoint: https://your-navidrome.com/rest/stream?id=.... Relative URLs (e.g., /rest/stream) will fail because the Chromecast resolves URLs relative to its own environment, not the sender's page.21  
* **contentType:** Must accurately reflect the MIME type (e.g., audio/flac, audio/mp3). Mismatched types (e.g., sending audio/mp3 for a FLAC file) can cause the receiver's decoder to error out immediately.  
* **metadata:** Populate chrome.cast.media.MusicTrackMediaMetadata. Fields like title, artist, albumName, and images (cover art) should be mapped from the Navidrome song object. Note that images must also use absolute URLs accessible from the public internet or the local LAN (if the Cast device is on the same subnet).22

#### **3.2.3 State Synchronization**

The RemotePlayerController is the mechanism for two-way binding. The React app must listen for CURRENT\_TIME\_CHANGED, PLAYER\_STATE\_CHANGED, and IS\_MUTED\_CHANGED events from this controller.

* **The Seek Bar Challenge:** Unlike local playback where timeUpdate fires rapidly, Cast events may be throttled to save bandwidth. The frontend implementation of the progress bar needs to interpolate time between events to ensure smooth movement, or accept a "steppy" update frequency. The RemotePlayerController provides getSeekableRange() which is essential for determining if the file is fully buffered or being transcoded in real-time (which might affect seekability).15

### **3.3 Receiver Application Considerations**

While the Default Receiver is the easiest path, it has limitations regarding **Authentication**.

* **Token Passing:** The Default Receiver does not support custom HTTP headers (like X-ND-Authorization). Therefore, authentication credentials *must* be embedded in the query parameters of the contentId URL (u=user\&p=enc:password... or u=user\&t=token\&s=salt...).21  
* **Security Implication:** Since stream URLs with embedded tokens might be logged or cached, the Navidrome implementation should ideally use short-lived tokens if the Subsonic API extension supports them, or rely on the standard salted token auth. The implementation must ensure that ssl is enforced (HTTPS), as sending credentials in a URL over cleartext HTTP is a severe vulnerability. Furthermore, modern Chromecast devices reject non-HTTPS media streams in many contexts.24

### **3.4 Media Loading and Metadata Mapping**

A specific issue highlighted in the research is the handling of metadata and queueing. When casting an album, the standard behavior is to load one track. However, a robust music player should support a queue.

* **Queue Load Request:** The Cast SDK supports loading a queue of items (queueLoadRequest). The Navidrome frontend, when playing an album or playlist, should construct a QueueData object containing QueueItems for the tracks. This allows the Chromecast to pre-buffer the next song, enabling gapless (or near-gapless) playback, which is a significant quality-of-life improvement over loading tracks one by one from the sender.23  
* **Image CORS:** The Chromecast will attempt to fetch album art from the URLs provided in the metadata. If the Navidrome server does not serve images with Access-Control-Allow-Origin: \*, the album art will fail to load on the TV screen. This is distinct from the stream CORS issue but equally important for UX.25

## ---

**4\. Apple AirPlay: Protocol and Implementation**

AirPlay integration offers a stark contrast to Google Cast. While Cast is an application-layer protocol involving complex SDKs, AirPlay on the web is a browser-feature integration. It relies on the browser (specifically Safari) to handle the discovery and transmission of media.

### **4.1 The AirPlay Stack**

AirPlay operates via mDNS (Bonjour) for discovery and RTSP/RAOP for streaming. However, in a web context, developers do not interact with these low-level protocols. Instead, they interact with the WebKit media interface.

* **Platform Exclusivity:** Native AirPlay support for web content is currently exclusive to Apple devices (macOS, iOS, iPadOS). Chrome and Firefox on Windows or Linux do not have access to the OS-level AirPlay stack via standard web APIs.26 The implementation must effectively be "Safari-aware."

### **4.2 Browser APIs and Availability**

There are two primary APIs for AirPlay, and the implementation must account for the deprecation and support lifecycle of each.

#### **4.2.1 The Proprietary WebKit API (WebKitPlaybackTargetAvailabilityEvent)**

This is the legacy but most reliable API for Safari.

* **Detection:** The React component (specifically the hook useAirPlay) checks for window.WebKitPlaybackTargetAvailabilityEvent. If present, it attaches an event listener to the audio element:  
  JavaScript  
  audioElement.addEventListener('webkitplaybacktargetavailabilitychanged', (event) \=\> {  
      if (event.availability \=== 'available') {  
          setShowAirPlayButton(true);  
      }  
  });

* **Triggering:** The AirPlay picker is a system menu. It cannot be rendered by the app; it must be invoked. The application calls audioElement.webkitShowPlaybackTargetPicker(). **Crucial Requirement:** This method calls for a "user gesture" (e.g., a direct click event). It cannot be called programmatically on load. The React event handler for the AirPlay button must call this method directly.28

#### **4.2.2 The Remote Playback API (W3C Standard)**

This is the modern standard intended to unify Cast and AirPlay (HTMLMediaElement.remote).

* **Status:** Support is fragmented. Chrome on Android supports it (routing to Cast). Safari supports it (routing to AirPlay). Chrome on Desktop supports it in newer versions (121+) but often lacks the backend to actually discover AirPlay devices without specific flags or extensions.30  
* **Strategy:** The robust approach is to implement a fallback chain. Try audio.remote.watchAvailability() first. If that API is undefined, fall back to the WebKit proprietary event. This ensures future compatibility while maintaining current functionality on macOS/iOS.26

### **4.3 Content Delivery and Formats**

A critical distinction in AirPlay is how the audio reaches the speaker.

* **Method A: Streaming (Screen Mirroring logic):** The browser downloads the audio file, decodes it, and re-encodes it (typically to ALAC or AAC) to send to the speaker. This happens if the target device cannot reach the stream URL directly (e.g., local file, authenticated stream behind a firewall the speaker can't navigate).  
* **Method B: Handoff (URL Passing):** If the content is a standard URL, AirPlay 2 can pass the URL to the speaker (e.g., HomePod), which then fetches the stream directly.

The Codec Dilemma (ALAC vs. AAC):  
Research indicates that AirPlay 2 streams from Apple Music can be lossy (AAC 256kbps) in certain configurations, even if the source is ALAC.31 For a Navidrome implementation, we largely rely on the browser's behavior.

* **Best Practice:** Navidrome should serve the highest quality format available (FLAC/ALAC). Safari supports FLAC in the audio tag. If Safari can decode it, it can stream it to AirPlay.  
* **Handoff Requirements:** To enable Handoff (which saves battery on the user's device and potentially allows higher quality), the Navidrome server must use a **valid, trusted SSL certificate**. Self-signed certificates often cause Handoff to fail, forcing the device to fall back to Method A (decoding and re-streaming), which is battery-intensive. The report must emphasize the necessity of Let's Encrypt or similar trusted CAs for optimal AirPlay performance.33

## ---

**5\. Backend Engineering: Supporting Remote Clients**

The frontend work is only half the battle. The Go backend must be tuned to serve these remote, automated clients which behave differently than a standard web browser.

### **5.1 Cross-Origin Resource Sharing (CORS) Configuration**

This is the single most common failure point for Chromecast integration. A Chromecast running the default receiver behaves like a web page hosted on a Google domain (.gstatic.com or similar). When it requests a stream from your-navidrome.com, the browser engine on the Chromecast enforces CORS.  
Required go-chi Middleware Configuration:  
The server/server.go file (or wherever middleware is defined) typically uses go-chi/cors.34 The configuration must be explicit.

* **Allowed Origins:** While \* works for public APIs, complex setups with cookies or credentials might require specific origins. However, for Cast, \* is the standard recommendation because the receiver's origin can vary.  
* **Exposed Headers:** The Access-Control-Expose-Headers configuration is critical. You **must** expose Content-Length, Content-Range, and Content-Type.  
  * *Why?* The Chromecast media player uses Content-Length and Content-Range to determine the duration of the file and to support seeking. If Content-Range is hidden (which is the default in many CORS configurations), the Chromecast assumes the stream is a live broadcast (infinite duration). The user will see a "Live" badge on their TV, and the seek bar will be disabled or broken.25

**Code Abstraction (Go):**

Go

// Conceptual Go-Chi Middleware  
cors.New(cors.Options{  
    AllowedOrigins:  string{"\*"},  
    AllowedMethods:  string{"GET", "OPTIONS", "HEAD"},  
    AllowedHeaders:  string{"Range", "Content-Type", "Accept", "X-ND-Authorization"},  
    ExposedHeaders:  string{"Content-Length", "Content-Range", "Content-Type"},  
    AllowCredentials: true,  
    MaxAge:           300,  
})

### **5.2 Authentication Strategies for Streaming**

As noted in the receiver section, custom headers are not an option for the Default Receiver. The backend must support token-based authentication in the query string for the /stream endpoint.

* **Subsonic Legacy:** The Subsonic API standardizes u (username) \+ t (token) \+ s (salt). Navidrome already supports this.  
* **Security Hardening:** Ensure that the token validation logic in the backend is efficient. Since casting might trigger multiple requests (metadata, cover art, stream chunks), the authentication middleware should be performant.  
* **JWT in URL:** Navidrome also supports JWT. Passing the JWT in the query parameter (e.g., ?jwt=...) is a valid alternative if the Subsonic token generation is cumbersome on the frontend, provided the backend is configured to look for the token in the URL as a fallback to the Authorization header.35

### **5.3 Transcoding Pipeline Enhancements**

Chromecast Audio supports FLAC (up to 96kHz/24-bit).36 However, network conditions vary.

* **Intelligent Transcoding:** The implementation should allow the user to select a "Cast Quality" profile in the settings.  
* **Format Parameter:** The frontend should append \&format=mp3 or \&maxBitRate=320 to the stream URL based on this setting. The backend ffmpeg pipeline already handles this logic.  
* **Latency:** Transcoding introduces startup latency. The backend should utilize the TranscodingCacheSize configuration effectively to cache converted chunks, ensuring that seeking or re-playing a track is instantaneous.37

## ---

**6\. Frontend Engineering: React Component Strategy**

The modernization of the frontend requires a disciplined approach to state management to handle the complexity of multiple playback targets.

### **6.1 State Management: Context vs. Redux**

The research highlights a debate between Redux and Context.39 For the specific use case of a Media Player, **React Context** combined with **useReducer** is the recommended approach for the Navidrome modernization.

* **Rationale:** The state (playing/paused, time, volume, current track) is global but manageable. Redux adds significant boilerplate. A PlayerContext can expose the state and dispatch actions (PLAY, PAUSE, SET\_DEVICE).  
* **Context Structure:**  
  JavaScript  
  const PlayerContext \= createContext({  
      playbackState: 'IDLE', // IDLE, BUFFERING, PLAYING, PAUSED  
      activeDevice: 'LOCAL', // LOCAL, CAST, AIRPLAY, SONOS  
      currentTrack: null,  
      //... methods  
  });

### **6.2 Custom Hooks for Device Abstraction**

To keep the UI components (like PlayerBar.js) clean, logic should be encapsulated in hooks.

* **useRemotePlayback:** A unifying hook that aggregates useCastSender, useAirPlay, and useSonos. It exposes a generic connect(deviceType) function.  
* **useCastSender Implementation Details:** This hook should manage the loading of the script (using a useEffect to inject the tag if missing) and maintain the CastContext reference. It needs to handle the cleanup of event listeners (removeEventListener) when the component unmounts to prevent memory leaks, which are common in SPAs interacting with external SDKs.16

### **6.3 UI/UX Integration Patterns**

* **Unified Connect Button:** Instead of cluttering the interface with distinct icons for Cast, AirPlay, and Sonos, implement a single "Connect Devices" button (icon: SpeakerGroup or similar). Clicking this opens a popover list of available targets.  
* **Conditional Rendering:**  
  * *Google Cast:* Show if cast.framework.CastContext.getCastState()\!== NO\_DEVICES\_AVAILABLE.  
  * *AirPlay:* Show if window.WebKitPlaybackTargetAvailabilityEvent is available.  
  * *Sonos:* Show if the Sonos discovery endpoint returns devices.  
* **Volume Control:** When casting, the local volume slider must control the *remote* device. The useRemotePlayback hook should proxy the setVolume call to remotePlayer.setVolumeLevel() (Cast) or the Sonos API. For AirPlay, the slider typically becomes read-only or controls system volume, as the browser delegates control to the OS.19

## ---

**7\. Comparative Analysis: Sonos, Cast, and AirPlay**

To aid the engineering team, we compare the integration efforts. The user is already working on Sonos, which serves as a baseline.

| Feature | Sonos (Current Work) | Google Cast | Apple AirPlay |
| :---- | :---- | :---- | :---- |
| **Discovery** | Backend/UPnP (usually) | Frontend SDK (Auto) | Browser/OS Native |
| **Control Logic** | Direct HTTP/SOAP calls from Backend or Proxy | JavaScript SDK (Sender API) | Browser Event Hooks |
| **Stream Source** | Pulls from URL (needs public/local IP) | Pulls from URL (needs public/local IP) | Pushed by Device (Mirroring) or Pull (Handoff) |
| **Auth Handling** | Specific Headers or URL tokens | URL tokens only (Default Receiver) | Cookies or URL tokens |
| **CORS** | Generally permissive | **Strictly Enforced** | N/A (Handled by OS/Browser) |
| **Format Support** | Broad (FLAC support varies by model) | Broad (FLAC/AAC/MP3) | ALAC/AAC (Managed by Safari) |
| **State Sync** | Polling or Event Subscription | Event Bus (RemotePlayerController) | Native Events |

**Key Insight:** Sonos and Chromecast are similar in that they both "pull" the stream from the server. AirPlay is unique as the browser often "pushes" the data. The architectural work done for Sonos regarding stream URL generation (embedded auth tokens) is directly reusable for Chromecast.

## ---

**8\. Operational Risks and Mitigation Strategies**

### **8.1 The HTTPS/Mixed Content Problem**

Chromecast requires the receiver to be HTTPS. If the receiver is HTTPS, it generally cannot fetch HTTP content (Mixed Content).

* **Risk:** Many self-hosted Navidrome users run on HTTP (http://192.168.1.10:4533).  
* **Mitigation:** The documentation must explicitly state that casting requires Navidrome to be served over HTTPS (via Reverse Proxy like Caddy or Nginx with Let's Encrypt). Alternatively, advanced users can use a Custom Receiver that allows HTTP, but this is a high-friction path.33

### **8.2 Network Isolation (mDNS)**

Both AirPlay and Cast rely on mDNS (Multicast DNS) for discovery.

* **Risk:** Docker containers often run in bridge mode, isolating them from the host's mDNS broadcasts.  
* **Mitigation:** For the *backend* discovery (Sonos), the container needs network\_mode: host. For Chromecast and AirPlay, discovery happens on the *client* (the browser), so the Docker network mode is irrelevant for discovery, *but* the stream URL generated by the backend must be accessible by the device. If the backend generates http://localhost:4533/stream, the Chromecast will fail.  
* **Fix:** Ensure the ND\_BASEURL or external URL configuration in Navidrome is set to the LAN IP or public domain, not localhost.37

### **8.3 "Gapless" Playback Limitations**

While Navidrome supports gapless playback locally, casting protocols introduce latency.

* **Risk:** A delay of 1-2 seconds between tracks as the Cast device loads the next URL.  
* **Mitigation:** Use the Chromecast queueLoadRequest to send the *next* song to the device while the current one is playing. This allows the Chromecast to buffer the next track in the background, significantly reducing the gap.23

## ---

**9\. Conclusion**

The integration of Chromecast and AirPlay transforms Navidrome from a personal music locker into a central home audio hub. The implementation path is clear but requires precision in the backend configuration to satisfy the strict security and networking requirements of these protocols.  
By leveraging the existing Subsonic API structure for stream generation and adopting a modern, hook-based React architecture for the frontend, the engineering team can deliver a seamless casting experience. The immediate priority should be the **backend CORS configuration** and **URL-based authentication** logic, as these are prerequisites for any successful Cast test. Following this, the creation of the useCastSender and useAirPlay hooks will encapsulate the complexity, ensuring the Navidrome codebase remains clean, maintainable, and ready for the next generation of streaming devices.

#### **Works cited**

1. Development Environment \- Navidrome, accessed on December 31, 2025, [https://www.navidrome.org/docs/developers/dev-environment/](https://www.navidrome.org/docs/developers/dev-environment/)  
2. Build from sources \- Navidrome, accessed on December 31, 2025, [https://www.navidrome.org/docs/installation/build-from-source/](https://www.navidrome.org/docs/installation/build-from-source/)  
3. Subsonic API Compatibility \- Navidrome, accessed on December 31, 2025, [https://www.navidrome.org/docs/developers/subsonic-api/](https://www.navidrome.org/docs/developers/subsonic-api/)  
4. subsonic package \- github.com/delucks/go-subsonic \- Go Packages, accessed on December 31, 2025, [https://pkg.go.dev/github.com/delucks/go-subsonic](https://pkg.go.dev/github.com/delucks/go-subsonic)  
5. stream \- OpenSubsonic, accessed on December 31, 2025, [https://opensubsonic.netlify.app/docs/endpoints/stream/](https://opensubsonic.netlify.app/docs/endpoints/stream/)  
6. navidrome/navidrome: ☁️ Your Personal Streaming Service \- GitHub, accessed on December 31, 2025, [https://github.com/navidrome/navidrome](https://github.com/navidrome/navidrome)  
7. Upgrade my docker to .49.1 and editing playlist causes a panic : r/navidrome \- Reddit, accessed on December 31, 2025, [https://www.reddit.com/r/navidrome/comments/10y9jl7/upgrade\_my\_docker\_to\_491\_and\_editing\_playlist/](https://www.reddit.com/r/navidrome/comments/10y9jl7/upgrade_my_docker_to_491_and_editing_playlist/)  
8. Navidrome Overview, accessed on December 31, 2025, [https://www.navidrome.org/docs/overview/](https://www.navidrome.org/docs/overview/)  
9. Navidrome, accessed on December 31, 2025, [https://www.navidrome.org/](https://www.navidrome.org/)  
10. navidrome-music-player \- A CDN for npm and GitHub \- jsDelivr, accessed on December 31, 2025, [https://www.jsdelivr.com/package/npm/navidrome-music-player](https://www.jsdelivr.com/package/npm/navidrome-music-player)  
11. react-music-player is unmaintaned · Issue \#1651 \- GitHub, accessed on December 31, 2025, [https://github.com/navidrome/navidrome/issues/1651](https://github.com/navidrome/navidrome/issues/1651)  
12. React Hooks and Functional Components | by Zach Landis \- Medium, accessed on December 31, 2025, [https://medium.com/@zachlandis91/react-hooks-and-functional-components-25a0bc00e023](https://medium.com/@zachlandis91/react-hooks-and-functional-components-25a0bc00e023)  
13. Why functional component/hooks were introduced in reactjs if class components was working fine. \- Reddit, accessed on December 31, 2025, [https://www.reddit.com/r/reactjs/comments/179221w/why\_functional\_componenthooks\_were\_introduced\_in/](https://www.reddit.com/r/reactjs/comments/179221w/why_functional_componenthooks_were_introduced_in/)  
14. Feature requests : r/navidrome \- Reddit, accessed on December 31, 2025, [https://www.reddit.com/r/navidrome/comments/12v5fom/feature\_requests/](https://www.reddit.com/r/navidrome/comments/12v5fom/feature_requests/)  
15. Integrate Cast SDK into Your Web Sender App \- Google for Developers, accessed on December 31, 2025, [https://developers.google.com/cast/docs/web\_sender/integrate](https://developers.google.com/cast/docs/web_sender/integrate)  
16. react-cast-sender \- npm, accessed on December 31, 2025, [https://www.npmjs.com/package/react-cast-sender](https://www.npmjs.com/package/react-cast-sender)  
17. Build a Custom Web Receiver | Cast \- Google for Developers, accessed on December 31, 2025, [https://developers.google.com/cast/codelabs/cast-receiver](https://developers.google.com/cast/codelabs/cast-receiver)  
18. Building Your Own Hooks \- React, accessed on December 31, 2025, [https://legacy.reactjs.org/docs/hooks-custom.html](https://legacy.reactjs.org/docs/hooks-custom.html)  
19. Sender App | Cast \- Google for Developers, accessed on December 31, 2025, [https://developers.google.com/cast/docs/design\_checklist/sender](https://developers.google.com/cast/docs/design_checklist/sender)  
20. CastSession \- React Native Google Cast, accessed on December 31, 2025, [https://react-native-google-cast.github.io/docs/api/classes/castsession](https://react-native-google-cast.github.io/docs/api/classes/castsession)  
21. Subsonic API, accessed on December 31, 2025, [https://www.subsonic.org/pages/api.jsp](https://www.subsonic.org/pages/api.jsp)  
22. Send custom metadata on a custom Chromecast receiver \- Bitmovin Community, accessed on December 31, 2025, [https://community.bitmovin.com/t/send-custom-metadata-on-a-custom-chromecast-receiver/2172](https://community.bitmovin.com/t/send-custom-metadata-on-a-custom-chromecast-receiver/2172)  
23. Google cast receiver framework (CAF) queueing images \- Stack Overflow, accessed on December 31, 2025, [https://stackoverflow.com/questions/49497898/google-cast-receiver-framework-caf-queueing-images](https://stackoverflow.com/questions/49497898/google-cast-receiver-framework-caf-queueing-images)  
24. Setup for Developing with the Cast Application Framework (CAF) for Web, accessed on December 31, 2025, [https://developers.google.com/cast/docs/web\_sender](https://developers.google.com/cast/docs/web_sender)  
25. \[Bug\]: setting Access-Control-Allow-Origin \-Header has to be configurable \#3660 \- GitHub, accessed on December 31, 2025, [https://github.com/navidrome/navidrome/issues/3660](https://github.com/navidrome/navidrome/issues/3660)  
26. Remote Playback API \- MDN Web Docs, accessed on December 31, 2025, [https://developer.mozilla.org/en-US/docs/Web/API/Remote\_Playback\_API](https://developer.mozilla.org/en-US/docs/Web/API/Remote_Playback_API)  
27. Remote Playback API \- Chrome Platform Status, accessed on December 31, 2025, [https://chromestatus.com/feature/5778318691401728](https://chromestatus.com/feature/5778318691401728)  
28. Adding an AirPlay button to your Safari media controls | Apple ..., accessed on December 31, 2025, [https://developer.apple.com/documentation/webkitjs/adding\_an\_airplay\_button\_to\_your\_safari\_media\_controls](https://developer.apple.com/documentation/webkitjs/adding_an_airplay_button_to_your_safari_media_controls)  
29. Airplay with Custom html5 controls \- Stack Overflow, accessed on December 31, 2025, [https://stackoverflow.com/questions/13655237/airplay-with-custom-html5-controls](https://stackoverflow.com/questions/13655237/airplay-with-custom-html5-controls)  
30. Remote playback | Can I use... Support tables for HTML5, CSS3, etc, accessed on December 31, 2025, [https://caniuse.com/wf-remote-playback](https://caniuse.com/wf-remote-playback)  
31. What is AirPlay 2? How it works, and what speakers and devices support it \- What Hi-Fi?, accessed on December 31, 2025, [https://www.whathifi.com/advice/apple-airplay-2-everything-you-need-to-know](https://www.whathifi.com/advice/apple-airplay-2-everything-you-need-to-know)  
32. Hopefully this synopsis clarifies it for more people : r/AppleMusic \- Reddit, accessed on December 31, 2025, [https://www.reddit.com/r/AppleMusic/comments/1fwt0ei/hopefully\_this\_synopsis\_clarifies\_it\_for\_more/](https://www.reddit.com/r/AppleMusic/comments/1fwt0ei/hopefully_this_synopsis_clarifies_it_for_more/)  
33. Security Considerations | Navidrome, accessed on December 31, 2025, [https://www.navidrome.org/docs/usage/security/](https://www.navidrome.org/docs/usage/security/)  
34. go-chi/cors: CORS net/http middleware for Go \- GitHub, accessed on December 31, 2025, [https://github.com/go-chi/cors](https://github.com/go-chi/cors)  
35. Vulnerability Summary for the Week of December 18, 2023 | CISA, accessed on December 31, 2025, [https://www.cisa.gov/news-events/bulletins/sb23-360](https://www.cisa.gov/news-events/bulletins/sb23-360)  
36. Chromecast audio specifications \- Streaming Help, accessed on December 31, 2025, [https://support.google.com/chromecast/answer/6279377?hl=en](https://support.google.com/chromecast/answer/6279377?hl=en)  
37. Navidrome Configuration Options, accessed on December 31, 2025, [https://www.navidrome.org/docs/usage/configuration-options/](https://www.navidrome.org/docs/usage/configuration-options/)  
38. ffmpeg question/issue : r/navidrome \- Reddit, accessed on December 31, 2025, [https://www.reddit.com/r/navidrome/comments/imdlt7/ffmpeg\_questionissue/](https://www.reddit.com/r/navidrome/comments/imdlt7/ffmpeg_questionissue/)  
39. I'm new to using global state management, is it ok to use context and redux? : r/reactjs, accessed on December 31, 2025, [https://www.reddit.com/r/reactjs/comments/1eh1i8u/im\_new\_to\_using\_global\_state\_management\_is\_it\_ok/](https://www.reddit.com/r/reactjs/comments/1eh1i8u/im_new_to_using_global_state_management_is_it_ok/)  
40. State Management : Context API vs Redux \- Medium, accessed on December 31, 2025, [https://medium.com/@rashmipatil24/state-management-in-react-b4b2e8c6cb9d](https://medium.com/@rashmipatil24/state-management-in-react-b4b2e8c6cb9d)