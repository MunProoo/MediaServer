package main

import (
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/deepch/vdk/format/rtmp"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtspv2"
	"log"
)

// StreamServerRunStreamDo stream run do mux
func StreamServerRunStreamDo(streamID string, channelID string, streamName string) {
	var status int
	defer func() {
		//TODO fix it no need unlock run if delete stream
		if status != 2 {
			Storage.StreamChannelUnlock(streamID, channelID)
		}
	}()
	for {
		log.Printf("[INFO] [core] [StreamServerRunStreamDo] Run stream: stream=%s", streamName)
		opt, err := Storage.StreamChannelControl(streamID, channelID)
		if err != nil {
			log.Printf("[ERROR] [core] [StreamServerRunStreamDo] failed to get stream channel: stream=%s error=%v", streamName, err)
			return
		}
		// 녹화 상태가 ON이면 클라이언트 없어도 스트림 유지
		if opt.OnDemand && !opt.OnRecording && !Storage.ClientHas(streamID, channelID) {
			log.Printf("[WARN] [core] [StreamServerRunStreamDo] Stop stream no client: stream=%s", streamName)
			return
		}
		status, err = StreamServerRunStream(streamID, channelID, opt, streamName)
		if status > 0 {
			log.Printf("[WARN] [core] [StreamServerRunStreamDo] Stream exit by signal or not client: stream=%s", streamName)
			return
		}
		if err != nil {
			log.Printf("[WARN] [core] [StreamServerRunStreamDo] Stream error restart stream: stream=%s error=%v", streamName, err)
		}
		time.Sleep(2 * time.Second)

	}
}

// StreamServerRunStream core stream
func StreamServerRunStream(streamID, channelID string, opt *ChannelST, streamName string) (int, error) {
	if url, err := url.Parse(opt.URL); err == nil && strings.ToLower(url.Scheme) == "rtmp" {
		return StreamServerRunStreamRTMP(streamID, channelID, opt, streamName)
	}
	keyTest := time.NewTimer(20 * time.Second)
	checkClients := time.NewTimer(20 * time.Second)
	var start bool
	var fps int
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{
		URL:                opt.URL,
		InsecureSkipVerify: opt.InsecureSkipVerify,
		DisableAudio:       !opt.Audio,
		DialTimeout:        3 * time.Second,
		ReadWriteTimeout:   15 * time.Second, // RTP 패킷 받아야하는 타임. 기존 5초
		Debug:              opt.Debug,
		OutgoingProxy:      true,
	})
	if err != nil {
		return 0, err
	}
	Storage.StreamChannelStatus(streamID, channelID, ONLINE)

	// on_recording이 ON이면 자동으로 녹화 시작
	if opt.OnRecording {
		log.Printf("[INFO] [core] [StreamServerRunStream] recording_status ON - 자동 녹화 시작: stream=%s", streamName)

		if err := Storage.StartRecording(streamID, channelID); err != nil {
			log.Printf("[ERROR] [core] [StreamServerRunStream] 자동 녹화 시작 실패: stream=%s error=%v", streamName, err)
		}
	}

	defer func() {
		RTSPClient.Close()
		Storage.StreamChannelStatus(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()
	var WaitCodec bool
	/*
		Example wait codec
	*/
	if RTSPClient.WaitCodec {
		WaitCodec = true
	} else {
		if len(RTSPClient.CodecData) > 0 {
			Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
		}
	}
	log.Printf("[INFO] [core] [StreamServerRunStream] Success connection RTSP: stream=%s", streamName)
	var ProbeCount int
	var ProbeFrame int
	var ProbePTS time.Duration
	Storage.NewHLSMuxer(streamID, channelID)
	defer Storage.HLSMuxerClose(streamID, channelID)
	for {
		select {
		//Check stream have clients
		case <-checkClients.C:
			// 클라이언트 없어짐. 녹화 옵션 off로 바뀜. -> 스트림 종료
			if opt.OnDemand && !opt.OnRecording && !Storage.ClientHas(streamID, channelID) {
				return 1, ErrorStreamNoClients
			}
			checkClients.Reset(20 * time.Second)
		//Check stream send key
		case <-keyTest.C:
			return 0, ErrorStreamNoVideo
		//Read core signals
		case signals := <-opt.signals:
			switch signals {
			case SignalStreamStop:
				return 2, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				return 0, ErrorStreamRestart
			case SignalStreamClient:
				return 1, ErrorStreamNoClients
			}
		//Read rtsp signals
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
				WaitCodec = false
			case rtspv2.SignalStreamRTPStop:
				return 0, ErrorStreamStopRTSPSignal
			}
		case packetRTP := <-RTSPClient.OutgoingProxyQueue:
			Storage.StreamChannelCastProxy(streamID, channelID, packetRTP)
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if WaitCodec {
				continue
			}

			if packetAV.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
				if preKeyTS > 0 {
					Storage.StreamHLSAdd(streamID, channelID, Seq, packetAV.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = packetAV.Time
			}
			Seq = append(Seq, packetAV)
			Storage.StreamChannelCast(streamID, channelID, packetAV)
			/*
			   HLS LL Test
			*/
			if packetAV.IsKeyFrame && !start {
				start = true
			}
			/*
				FPS mode probe
			*/
			if start {
				ProbePTS += packetAV.Duration
				ProbeFrame++
				if packetAV.IsKeyFrame && ProbePTS.Seconds() >= 1 {
					ProbeCount++
					if ProbeCount == 2 {
						fps = int(math.Round(float64(ProbeFrame) / ProbePTS.Seconds()))
					}
					ProbeFrame = 0
					ProbePTS = 0
				}
			}
			if start && fps != 0 {
				//TODO fix it
				packetAV.Duration = time.Duration((float32(1000)/float32(fps))*1000*1000) * time.Nanosecond
				Storage.HlsMuxerSetFPS(streamID, channelID, fps)
				Storage.HlsMuxerWritePacket(streamID, channelID, packetAV)
			}
		}
	}
}
func StreamServerRunStreamRTMP(streamID, channelID string, opt *ChannelST, streamName string) (int, error) {
	keyTest := time.NewTimer(20 * time.Second)
	checkClients := time.NewTimer(20 * time.Second)
	OutgoingPacketQueue := make(chan *av.Packet, 1000)
	Signals := make(chan int, 100)
	var start bool
	var fps int
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet

	conn, err := rtmp.DialTimeout(opt.URL, 3*time.Second)
	if err != nil {
		return 0, err
	}

	Storage.StreamChannelStatus(streamID, channelID, ONLINE)
	defer func() {
		conn.Close()
		Storage.StreamChannelStatus(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()
	var WaitCodec bool

	codecs, err := conn.Streams()
	if err != nil {
		return 0, err
	}
	preDur := make([]time.Duration, len(codecs))
	Storage.StreamChannelCodecsUpdate(streamID, channelID, codecs, []byte{})

	log.Printf("[INFO] [core] [StreamServerRunStreamRTMP] Success connection RTSP: stream=%s", streamName)
	var ProbeCount int
	var ProbeFrame int
	var ProbePTS time.Duration
	Storage.NewHLSMuxer(streamID, channelID)
	defer Storage.HLSMuxerClose(streamID, channelID)

	go func() {
		for {
			ptk, err := conn.ReadPacket()
			if err != nil {
				break
			}
			OutgoingPacketQueue <- &ptk
		}
		Signals <- 1
	}()

	for {
		select {
		//Check stream have clients
		case <-checkClients.C:
			// 클라이언트 없어짐. 녹화 옵션 off로 바뀜. -> 스트림 종료
			if opt.OnDemand && !opt.OnRecording && !Storage.ClientHas(streamID, channelID) {
				return 1, ErrorStreamNoClients
			}
			checkClients.Reset(20 * time.Second)
		//Check stream send key
		case <-keyTest.C:
			return 0, ErrorStreamNoVideo
		//Read core signals
		case signals := <-opt.signals:
			switch signals {
			case SignalStreamStop:
				return 2, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				return 0, ErrorStreamRestart
			case SignalStreamClient:
				return 1, ErrorStreamNoClients
			}
		//Read rtsp signals
		case <-Signals:
			return 0, ErrorStreamStopRTSPSignal
		case packetAV := <-OutgoingPacketQueue:
			if preDur[packetAV.Idx] != 0 {
				packetAV.Duration = packetAV.Time - preDur[packetAV.Idx]
			}

			preDur[packetAV.Idx] = packetAV.Time

			if WaitCodec {
				continue
			}

			if packetAV.IsKeyFrame {
				keyTest.Reset(20 * time.Second)
				if preKeyTS > 0 {
					Storage.StreamHLSAdd(streamID, channelID, Seq, packetAV.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = packetAV.Time
			}
			Seq = append(Seq, packetAV)
			Storage.StreamChannelCast(streamID, channelID, packetAV)
			/*
			   HLS LL Test
			*/
			if packetAV.IsKeyFrame && !start {
				start = true
			}
			/*
				FPS mode probe
			*/
			if start {
				ProbePTS += packetAV.Duration
				ProbeFrame++
				if packetAV.IsKeyFrame && ProbePTS.Seconds() >= 1 {
					ProbeCount++
					if ProbeCount == 2 {
						fps = int(math.Round(float64(ProbeFrame) / ProbePTS.Seconds()))
					}
					ProbeFrame = 0
					ProbePTS = 0
				}
			}
			if start && fps != 0 {
				//TODO fix it
				packetAV.Duration = time.Duration((float32(1000)/float32(fps))*1000*1000) * time.Nanosecond
				Storage.HlsMuxerSetFPS(streamID, channelID, fps)
				Storage.HlsMuxerWritePacket(streamID, channelID, packetAV)
			}
		}
	}
}
