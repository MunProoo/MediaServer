# MediaServer Specification Document

Date: 2026-03-25

## 1. Recommended Specs by Number of Cameras (Key Conclusion)

The table below is a conservative recommendation based on:
**keeping 100 continuously active streams + playing them simultaneously in a WebRTC grid (1-2 viewers) + up to 6 concurrent recordings**.

### 1-1. Common Recommendations

- Network
  - Minimum: 1Gbps, Recommended: 2.5Gbps
- Disk I/O Stability
  - HDD can work alone, but if write latency/stuttering occurs, video quality may degrade starting from the storage segment onward.
- RAM Headroom
  - Because Go's channel buffers/packet queues may accumulate, having "spare RAM" directly affects stability.

### 1-2. Recommended Specs by Camera Count (Example)

| Number of cameras (streams) | CPU recommendation | RAM recommendation | Network recommendation | HDD recommendation (for 30 days) |
|---:|---|---|---|---|
| 10 ~ 24 | 6 cores / 12 threads or more | 16GB | 1Gbps | 4TB (with buffer for 6 concurrent recordings) |
| 25 ~ 49 | 8 cores / 16 threads or more | 32GB | 2.5Gbps | 6TB (with buffer / accounting for increases) |
| 50 ~ 79 | 12 cores / 24 threads or more | 48GB | 2.5Gbps | 8TB |
| 80 ~ 100 | 16 cores / 32 threads or more | 64GB | 2.5Gbps+ | 8TB ~ 12TB (when targeting up to 10 recordings) |

Explanation (why these values):
- CPU usage is not mainly driven by "video resolution conversion." It is consumed by internal replication/packet handling, Go runtime, encryption, and queue handling.
- Therefore, performance is strongly influenced by the combination of **concurrent WebRTC streaming output + concurrent recording processes (6 at a time)**.

---

## 2. Terms / Assumptions

- Number of cameras registered: **100**, channels: **1**
- Camera stream quality (approx.): **bitrate 2 Mbps**, resolution 1920x1080, 30fps
- WebRTC viewers (concurrent users): up to **1-2 people** (in a control-system grid, playing 100 streams simultaneously)
- Concurrent recordings: **keep at most 6** (if there is headroom, you can operate up to 10)
- Measured recording storage:
  - "A single device's recording capacity for 24 hours is about **15GB**"

Notes:
- WebRTC becomes the worst-case load when viewers actually keep all 100 streams playing continuously.
- Recording is based on video `copy`, so it is less of a "resolution conversion / transcoding CPU explosion." The main costs are process/packet handling, audio encoding, and HLS file generation/encryption.

---

## 3. Main Role of the Media Server (Based on Code/Behavior)

This media server performs the following three tasks for RTSP inputs.

1. **RTSP Replication (Fan-out)**
   Each stream connects to the camera `opt.URL` (original RTSP) once to receive packets, then distributes the packets internally as a **replicated stream** to multiple consumers (WebRTC/RTSP/etc.).

2. **RTSP Recording (HLS, ffmpeg-based)**
   Recording does not attach ffmpeg directly to the camera's original RTSP. Instead, it uses the server's internal RTSP:
   `rtsp://localhost:<RTSPPort>/<streamID>/<channelID>`
   as the ffmpeg input to generate HLS.
   Current settings:
   - video: `-c:v copy` (no re-encoding)
   - audio: `-c:a aac` (encoding)

3. **RTSP -> WebRTC Transmission**
   For each user request, the WebRTC muxer consumes the above **replicated stream (internal packet fan-out)** and transmits it to the browser.
   Therefore, WebRTC does not reconnect separately to the camera's original RTSP; it uses the internally replicated stream.

---

## 4. Network Recommendation Estimation

### 4-1. Camera -> Media Server Inbound

- 100 cameras x 2 Mbps = **200 Mbps**

### 4-2. Media Server -> Browser WebRTC Outbound (Worst Case)

Assume the "100 streams are played simultaneously" scenario.

- 1 viewer: 100 x 2 Mbps = **200 Mbps**
- 2 viewers: 100 x 2 Mbps = **400 Mbps**

### 4-3. Total (Recommended headroom including overhead)

- 1 viewer: 200 + 200 = **400 Mbps**
- 2 viewers: 200 + 400 = **600 Mbps**
- Headroom (overhead/retransmissions/other traffic): assume about 20-30%

Therefore, recommended:
- **Minimum:** 1Gbps
- **Recommended:** 2.5Gbps (safe margin)

---

## 5. Recording Storage Calculation (Measured: ~15GB per device per 24 hours)

### 5-1. Units

- Per device per day: **15GB**
- 6 concurrent recordings per day: 15GB x 6 = **90GB/day**
- 10 concurrent recordings per day: 15GB x 10 = **150GB/day**

### 5-2. Example retention days (default setting: retention_days=30)

#### (1) 6 concurrent recordings, 30 days

- 90GB/day x 30 = **2,700GB ≈ 2.7TB**
- HLS split/header/filesystem buffer (recommended 1.2x):
  - 2.7TB x 1.2 = **~3.24TB**

-> Recommended HDD: **4TB or more** (recommended 6TB including buffer)

#### (2) 10 concurrent recordings, 30 days

- 150GB/day x 30 = **4,500GB ≈ 4.5TB**
- With 1.2x buffer:
  - 4.5TB x 1.2 = **~5.4TB**

-> Recommended HDD: **8TB or more**

### 5-3. Additional Considerations (On-site variance)

- Recording size may vary depending on camera settings (bitrate changes) and scene complexity.
- Since recording uses encryption (HLS AES-128) and audio encoding, the "measured/real-world value" is the most accurate.
- Therefore, it is recommended to confirm real usage during the first 1-3 days and apply a correction factor.

---

## 6. Operational Recommended Checklist

1. Keep concurrent recordings at "6", and adjust if actual accumulated storage does not match the expected value (15GB/24h) after observing for 1-3 days.
2. Monitor NIC/disk/memory usage during peak times when WebRTC output increases.
3. For HDD, capacity is not the only important metric; continuous write performance matters too (RAID strategies, avoiding low-speed devices, etc.).

