package tts

import (
	"fmt"
	"io"
	"log"
	"time"

	edge_tts "github.com/wujunwei928/edge-tts-go/edge_tts"
)

// EdgeTTSProvider 使用 Edge-TTS 合成语音。
type EdgeTTSProvider struct {
	// MaxRetries 是合成失败后的最大重试次数（不含首次调用）。
	MaxRetries int
	// RetryDelay 是两次尝试之间的间隔。
	RetryDelay time.Duration
}

// NewEdgeTTSProvider 创建默认的 EdgeTTSProvider。
func NewEdgeTTSProvider() *EdgeTTSProvider {
	return &EdgeTTSProvider{
		MaxRetries: 3,
		RetryDelay: 200 * time.Millisecond,
	}
}

// Synthesize 合成语音并写入 w，失败时返回错误。
func (p *EdgeTTSProvider) Synthesize(text, voice string, w io.Writer) error {
	if voice == "" {
		voice = DefaultVoice
	}

	var lastErr error
	for attempt := 0; attempt <= p.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(p.RetryDelay)
		}

		err := p.doSynthesize(text, voice, w)
		if err == nil {
			return nil
		}
		lastErr = err
		log.Printf("Edge-TTS attempt %d failed: %v", attempt+1, err)
	}
	return fmt.Errorf("Edge-TTS failed after %d attempts: %w", p.MaxRetries+1, lastErr)
}

// doSynthesize 单次合成，内部调用 edge-tts-go。
func (p *EdgeTTSProvider) doSynthesize(text, voice string, w io.Writer) error {
	comm, err := edge_tts.NewCommunicate(
		text,
		edge_tts.SetVoice(voice),
		edge_tts.SetOutputFormat(edge_tts.OutputFormatMP3),
	)
	if err != nil {
		return err
	}

	audio, err := comm.Stream()
	if err != nil {
		return err
	}

	_, err = w.Write(audio)
	return err
}
