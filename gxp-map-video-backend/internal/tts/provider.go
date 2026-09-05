package tts

import "io"

// DefaultVoice 是 Edge-TTS 的默认中文语音。
const DefaultVoice = "zh-CN-XiaoxiaoNeural"

// TTSProvider 定义 TTS 服务接口，方便后续扩展降级方案（如 MOSS-TTS-Nano）。
type TTSProvider interface {
	Synthesize(text, voice string, w io.Writer) error
}
