package doubao

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// 协议常量
const (
	// Protocol version
	protocolVersion = 0x01

	// Header size (1 = 4 bytes)
	headerSize = 0x01

	// Message types
	msgTypeFullClient   = 0x01 // 客户端发送完整请求
	msgTypeAudioOnly    = 0x02 // 仅音频数据
	msgTypeFullServer   = 0x09 // 服务端完整响应
	msgTypeServerAck    = 0x0B // 服务端确认
	msgTypeServerError  = 0x0F // 服务端错误

	// Serialization
	serializationJSON = 0x01

	// Compression
	compressionNone = 0x00
	compressionGzip = 0x01

	// Event codes
	eventSessionStarted      = 150
	eventPodcastRoundStart   = 360
	eventPodcastRoundResp    = 361
	eventPodcastRoundEnd     = 362
	eventPodcastEnd          = 363
	eventSessionFinished     = 152
	eventUsageResponse       = 154
	eventFinishConnection    = 2
	eventConnectionFinished  = 52
)

// Frame 协议帧
type Frame struct {
	Version       byte
	HeaderSize    byte
	MessageType   byte
	Flags         byte
	Serialization byte
	Compression   byte
	Reserved      byte
	EventNumber   uint32
	SessionID     string
	Payload       []byte
}

// EncodeFrame 编码帧
func EncodeFrame(sessionID string, eventNumber uint32, payload interface{}) ([]byte, error) {
	// 序列化 payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Gzip 压缩
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(payloadBytes); err != nil {
		return nil, fmt.Errorf("failed to compress payload: %w", err)
	}
	gz.Close()
	payloadBytes = compressed.Bytes()

	// 构建帧
	sessionIDBytes := []byte(sessionID)
	totalLen := 4 + 4 + 4 + len(sessionIDBytes) + 4 + len(payloadBytes)

	buf := make([]byte, totalLen)
	offset := 0

	// Byte 0: version (4 bits) + header size (4 bits)
	buf[offset] = (protocolVersion << 4) | headerSize
	offset++

	// Byte 1: message type (4 bits) + flags (4 bits)
	buf[offset] = (msgTypeFullClient << 4) | 0x00
	offset++

	// Byte 2: serialization (4 bits) + compression (4 bits)
	buf[offset] = (serializationJSON << 4) | compressionGzip
	offset++

	// Byte 3: reserved
	buf[offset] = 0x00
	offset++

	// Bytes 4-7: event number (big endian)
	binary.BigEndian.PutUint32(buf[offset:], eventNumber)
	offset += 4

	// Bytes 8-11: session ID length
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(sessionIDBytes)))
	offset += 4

	// Session ID
	copy(buf[offset:], sessionIDBytes)
	offset += len(sessionIDBytes)

	// Payload length
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(payloadBytes)))
	offset += 4

	// Payload
	copy(buf[offset:], payloadBytes)

	return buf, nil
}

// EncodeFinishFrame 编码结束帧
func EncodeFinishFrame(sessionID string) []byte {
	sessionIDBytes := []byte(sessionID)
	totalLen := 4 + 4 + 4 + len(sessionIDBytes)

	buf := make([]byte, totalLen)
	offset := 0

	// Byte 0: version + header size
	buf[offset] = (protocolVersion << 4) | headerSize
	offset++

	// Byte 1: message type + flags
	buf[offset] = (msgTypeFullClient << 4) | 0x00
	offset++

	// Byte 2: serialization + compression (no compression for finish)
	buf[offset] = (serializationJSON << 4) | compressionNone
	offset++

	// Byte 3: reserved
	buf[offset] = 0x00
	offset++

	// Event number = 2 (FinishConnection)
	binary.BigEndian.PutUint32(buf[offset:], eventFinishConnection)
	offset += 4

	// Session ID length
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(sessionIDBytes)))
	offset += 4

	// Session ID
	copy(buf[offset:], sessionIDBytes)

	return buf
}

// DecodeFrame 解码帧
func DecodeFrame(data []byte) (*Frame, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("frame too short: %d bytes", len(data))
	}

	frame := &Frame{}
	offset := 0

	// Byte 0: version + header size
	frame.Version = (data[offset] >> 4) & 0x0F
	frame.HeaderSize = data[offset] & 0x0F
	offset++

	// Byte 1: message type + flags
	frame.MessageType = (data[offset] >> 4) & 0x0F
	frame.Flags = data[offset] & 0x0F
	offset++

	// Byte 2: serialization + compression
	frame.Serialization = (data[offset] >> 4) & 0x0F
	frame.Compression = data[offset] & 0x0F
	offset++

	// Byte 3: reserved
	frame.Reserved = data[offset]
	offset++

	// Bytes 4-7: event number
	frame.EventNumber = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	// Session ID length
	if len(data) < offset+4 {
		return frame, nil
	}
	sessionIDLen := binary.BigEndian.Uint32(data[offset:])
	offset += 4

	// Session ID
	if len(data) < offset+int(sessionIDLen) {
		return frame, nil
	}
	frame.SessionID = string(data[offset : offset+int(sessionIDLen)])
	offset += int(sessionIDLen)

	// 根据消息类型处理 payload
	if frame.MessageType == msgTypeAudioOnly {
		// 音频数据: sequence (4 bytes) + payload size (4 bytes) + payload
		if len(data) < offset+8 {
			return frame, nil
		}
		// sequence := binary.BigEndian.Uint32(data[offset:])
		offset += 4
		payloadSize := binary.BigEndian.Uint32(data[offset:])
		offset += 4
		if len(data) >= offset+int(payloadSize) {
			frame.Payload = data[offset : offset+int(payloadSize)]
		}
	} else {
		// 普通消息: payload size (4 bytes) + payload
		if len(data) < offset+4 {
			return frame, nil
		}
		payloadSize := binary.BigEndian.Uint32(data[offset:])
		offset += 4
		if len(data) >= offset+int(payloadSize) {
			payload := data[offset : offset+int(payloadSize)]

			// 解压缩
			if frame.Compression == compressionGzip && len(payload) > 0 {
				gr, err := gzip.NewReader(bytes.NewReader(payload))
				if err == nil {
					decompressed, err := io.ReadAll(gr)
					gr.Close()
					if err == nil {
						payload = decompressed
					}
				}
			}
			frame.Payload = payload
		}
	}

	return frame, nil
}

// PayloadResponse 响应 payload
type PayloadResponse struct {
	StatusCode    int     `json:"status_code"`
	StatusMessage string  `json:"status_message"`
	Speaker       string  `json:"speaker"`
	RoundID       int     `json:"round_id"`
	Text          string  `json:"text"`
	AudioDuration float64 `json:"audio_duration"`
}

// ParsePayload 解析 payload
func ParsePayload(data []byte) (*PayloadResponse, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var resp PayloadResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
