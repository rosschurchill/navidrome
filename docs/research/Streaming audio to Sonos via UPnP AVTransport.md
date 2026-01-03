# Streaming audio to Sonos via UPnP/AVTransport

**The key to successful Sonos streaming lies in three critical requirements**: proper HTTP headers (especially `Content-Length`), correct URI scheme selection (`x-rincon-mp3radio://` for radio-style streams since Sonos v6.4.2), and handling the rapid TRANSITIONING→PLAYING state change. While `CurrentURIMetaData` can technically be empty, providing minimal DIDL-Lite metadata ensures proper display. Error 714 (Illegal MIME-Type) typically indicates HTTP server header issues, while stuck TRANSITIONING states usually stem from format problems, missing Content-Length headers, or network accessibility issues.

## HTTP streaming server requirements are non-negotiable

Sonos speakers are demanding HTTP clients with specific expectations your server must satisfy.

### Content-Type headers by format

| Format | Required MIME Type |
|--------|-------------------|
| **FLAC** | `audio/flac` |
| **MP3** | `audio/mpeg` (preferred), `audio/mp3` |
| **AAC** | `audio/mp4`, `audio/aac` |
| **OGG Vorbis** | `application/ogg` |
| **WAV** | `audio/wav`, `audio/x-wav` |
| **WMA** | `audio/x-ms-wma` |

### Content-Length is mandatory for seeking

According to official Sonos documentation: "Your service **must** return an accurate `Content-Length` HTTP header. Sonos requires this to support seeking to any point in the file for formats where time position doesn't map linearly to byte positions." Without this header, seeking and scrubbing fail silently.

Sonos may send **HEAD requests** before GET requests specifically to discover Content-Length. Your server must respond correctly to both methods.

### Transfer-Encoding: chunked has format limitations

Chunked encoding works reliably only for **WAV and FLAC** formats. For MP3 streams, chunked encoding causes playback failures. The workaround for MP3 radio streams: use `x-rincon-mp3radio://` URI scheme instead of `http://`, which triggers different buffering behavior in the player.

For live streams without known length, set a fake Content-Length value and close the connection when finished—Sonos tolerates this pattern.

### Range request support determines seek functionality

Range requests are **essential** for resume-from-pause, seeking, and multi-part buffering. Your server must:

- Support `Range: bytes=start-` headers
- Return **206 Partial Content** (not 200) for partial requests
- Include `Content-Range: bytes start-end/total` header
- Return 416 for invalid range requests

**Critical**: CDNs returning 200 instead of 206 for partial content are incompatible with Sonos seeking.

Example range response:
```http
HTTP/1.1 206 Partial Content
Content-Type: audio/flac
Accept-Ranges: bytes
Content-Range: bytes 1000000-44999999/45000000
Content-Length: 44000000
```

### Connection handling expects HTTP 1.1 persistent connections

Sonos requires keep-alive connections—it makes repeated GET requests during buffering. The SMAPI timeout is 10 seconds maximum; streaming URL validity should persist for track duration. If your server closes the connection mid-stream, Sonos issues new Range requests to continue.

## URI schemes determine playback behavior

Since Sonos v6.4.2, plain `http://` URLs no longer work correctly for radio-style streams.

| Scheme | Use Case |
|--------|----------|
| `x-rincon-mp3radio://` | **Radio streams** - required for continuous streams since v6.4.2 |
| `http://`, `https://` | File downloads, discrete tracks (limited radio support) |
| `x-rincon-queue:RINCON_XXX#0` | Play from device queue |
| `x-rincon:RINCON_XXX` | Group with another speaker |
| `x-sonosapi-stream:` | TuneIn/music service streams |
| `x-file-cifs://` | SMB network shares |
| `x-sonos-http:` | Google Play, Napster services |

**Converting HTTP to radio scheme**:
```
Original: http://stream.example.com/radio.mp3
Required: x-rincon-mp3radio://stream.example.com/radio.mp3
```

In SoCo Python, use `force_radio=True` parameter to automatically convert the scheme. S1 and S2 systems share identical URI scheme requirements.

## Audio format specifications and constraints

### FLAC streaming limits

- **Sample rates**: 8, 11.025, 16, 22.05, 24, 32, 44.1, 48 kHz (maximum 48 kHz—**96/192 kHz not supported**)
- **Bit depth**: 24-bit max on S2, 16-bit max on S1
- **Frame size**: 32 KB maximum
- **Best practice**: Include seek table with 100 entries for 1% resolution seeking

### General format requirements

All formats require **mono or stereo only**—multichannel audio fails. MP3/AAC/OGG cap at **320 kbps**. For VBR MP3 files, include Xing header with TOC (Table of Contents) for seeking. Place metadata at front of file to minimize seek requests.

## SetAVTransportURI SOAP envelope structure

### Complete working SOAP request

```xml
POST /MediaRenderer/AVTransport/Control HTTP/1.1
Host: 192.168.1.100:1400
Content-Type: text/xml; charset="utf-8"
SOAPAction: "urn:schemas-upnp-org:service:AVTransport:1#SetAVTransportURI"

<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" 
            s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <CurrentURI>x-rincon-mp3radio://example.com/stream.mp3</CurrentURI>
      <CurrentURIMetaData></CurrentURIMetaData>
    </u:SetAVTransportURI>
  </s:Body>
</s:Envelope>
```

**Required elements**: `s:encodingStyle` attribute is mandatory. `InstanceID` must always be **0** for Sonos consumer devices. The endpoint is `/MediaRenderer/AVTransport/Control` on port **1400**.

### CurrentURIMetaData can be empty—with caveats

**Yes**, `CurrentURIMetaData` accepts empty string `""` or empty element `<CurrentURIMetaData/>`. Playback works without metadata for simple HTTP streams and queue-based playback.

**However**, for radio streams to display properly with title information, provide minimal DIDL-Lite:

```xml
<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" 
           xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" 
           xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" 
           xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
  <item id="R:0/0/0" parentID="R:0/0" restricted="true">
    <dc:title>My Radio Station</dc:title>
    <upnp:class>object.item.audioItem.audioBroadcast</upnp:class>
    <desc id="cdudn" nameSpace="urn:schemas-rinconnetworks-com:metadata-1-0/">SA_RINCON65031_</desc>
  </item>
</DIDL-Lite>
```

When embedding in SOAP, XML-escape all special characters: `<` → `&lt;`, `>` → `&gt;`, `"` → `&quot;`, `&` → `&amp;`.

## Error codes and their root causes

| Code | Name | Cause | Solution |
|------|------|-------|----------|
| **714** | Illegal MIME-Type | Server returns wrong/missing Content-Type | Fix HTTP headers; verify audio format support |
| **716** | Resource Not Found | URL unreachable from Sonos network | Check firewall, DNS, server accessibility |
| **718** | Invalid InstanceID | InstanceID ≠ 0 | Always use InstanceID=0 |
| **701** | Transition not available | Seek on non-seekable content or non-coordinator | Send to group coordinator; check content |
| **704** | Format not supported | Unsupported codec | Use MP3, AAC, FLAC, or WAV |
| **800** | Not a coordinator | Command sent to grouped speaker | Send to group coordinator only |

## Why TRANSITIONING never becomes PLAYING

This frustrating issue has multiple root causes discovered through community debugging.

### Race condition in event handling

Sonos sends two notifications in rapid succession: `TransportState=TRANSITIONING` followed immediately by `TransportState=PLAYING`. If your code processes these out of order (due to threading, async delays, or album art downloads), the state appears stuck in TRANSITIONING.

**Solution**: Treat TRANSITIONING as equivalent to PLAYING for UI purposes, or process only PLAYING events and ignore TRANSITIONING entirely.

### Sonos expects immediate data availability

Unlike players that buffer before starting, **Sonos starts playback at the first received byte**, expecting a large HTTP data burst. From philippe44's LMS-uPnP implementation: "If the player starts at the first received byte (like Sonos do), counting on the fact that because it's HTTP, the resource should be available...then there is only disappointment to expect."

Your server must respond immediately with audio data. Send **500ms+ of audio (or silence)** in the initial response before any pauses. If your source pauses, send silence frames to maintain the connection.

### Stream format rejection

Sonos silently fails on OGG Vorbis in many contexts despite nominal support. FLAC above 48 kHz fails. Missing Content-Length for MP3 causes indefinite buffering.

### Double request problem

Sonos sometimes requests the same stream twice. From philippe44: "Reject the second request, continue serving the first." Track this via connection state.

## bonob demonstrates SMAPI architecture, not direct UPnP

**Important architectural insight**: bonob implements SMAPI (Sonos Music API), not direct UPnP control. It registers as a music service, and Sonos natively handles all `SetAVTransportURI` and DIDL-Lite construction internally after receiving streaming URLs via `getMediaURI` SMAPI response.

bonob's HTTP streaming server returns:
```http
HTTP/1.1 200 OK
Content-Type: audio/flac
Accept-Ranges: bytes
Content-Length: 45000000
```

For seek/resume:
```http
HTTP/1.1 206 Partial Content
Content-Range: bytes 3480315-/4570936
```

Authentication uses custom headers (`bnbt`, `bnbk`) passed via SMAPI's `httpHeaders` element—Sonos forwards these verbatim to the streaming server.

## SoCo Python implementation reference

```python
from soco import SoCo

sonos = SoCo('192.168.1.102')

# Simple HTTP audio (empty metadata works)
sonos.play_uri('http://server:8000/audio.mp3')

# Radio stream with proper scheme and title
sonos.play_uri(
    'http://stream.example.com:8000/radio',
    title='My Radio Station',
    force_radio=True  # Converts to x-rincon-mp3radio://
)

# Full custom metadata
metadata = '''<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"
  xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/"
  xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
  <item id="R:0/0/0" parentID="R:0/0" restricted="true">
    <dc:title>Custom Track</dc:title>
    <upnp:class>object.item.audioItem.musicTrack</upnp:class>
  </item>
</DIDL-Lite>'''
sonos.play_uri('http://server:8000/track.flac', meta=metadata)
```

## Debugging and network traffic analysis

### Capture Sonos requests with Wireshark

Filter by Sonos IP and port 1400:
```
ip.addr==192.168.x.x && tcp.port==1400
```

Monitor `/MediaRenderer/AVTransport/Control` for SOAP commands and your streaming server for GET/HEAD requests with Range headers.

### Access Sonos logs directly

Navigate to `http://<SONOS_IP>:1400/support/review` for device logs. Application log at `/opt/log/anacapa.trace` shows transport events and errors.

### Required network ports

- **TCP 1400**: Primary UPnP control
- **TCP 80/443**: Audio streaming
- **UDP 1900**: SSDP discovery (multicast 239.255.255.250)
- **TCP 3400-3500**: UPnP event callbacks

### HTTPS certificate verification

Sonos S2 requires TLS 1.2 with valid certificates from recognized CAs. Self-signed certificates fail. Let's Encrypt works via DST Root CA X3 cross-signature. For local development, use HTTP—Sonos accepts unencrypted streams on local networks.

## Conclusion

Successful Sonos streaming requires satisfying three interconnected requirements. **First**, your HTTP server must provide `Content-Length` headers, support Range requests with 206 responses, and return correct MIME types—error 714 almost always indicates header problems. **Second**, for radio-style streams, use `x-rincon-mp3radio://` scheme since Sonos v6.4.2 broke plain HTTP radio support. **Third**, respond immediately with audio data since Sonos expects instant playback from HTTP sources.

The TRANSITIONING state issue is typically a race condition in your event handling code rather than a protocol problem—process PLAYING events and ignore TRANSITIONING, or treat them equivalently. Empty DIDL-Lite metadata works for basic playback but causes display issues; minimal metadata with just `dc:title` and `upnp:class` resolves this without complexity. For production implementations, study SoCo's SOAP envelope construction and philippe44's insights on Sonos buffering behavior—Sonos is uniquely aggressive about starting playback immediately and expects your server to deliver data without hesitation.