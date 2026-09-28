package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const sampleRate = 22050

type SoundBoard struct {
	ctx   *audio.Context
	ping  *audio.Player // 玩家脉冲：高频扫频
	mPing *audio.Player // 怪物脉冲：低频扫频
	step  *audio.Player // 脚步
	alert *audio.Player //怪物警觉低吼
	death *audio.Player // 死亡
}

func newSoundBoard() *SoundBoard {
	ctx := audio.NewContext(sampleRate)
	return &SoundBoard{
		ctx:   ctx,
		ping:  mustPlayer(ctx, chirp(900, 430, 0.28, 9)),
		mPing: mustPlayer(ctx, chirp(330, 170, 0.4, 7)),
		step:  mustPlayer(ctx, thump()),
		alert: mustPlayer(ctx, growl()),
		death: mustPlayer(ctx, chirp(420, 55, 0.7, 3)),
	}
}

func mustPlayer(ctx *audio.Context, samples []float64) *audio.Player {
	stream, err := wav.Decode(ctx, bytes.NewReader(encodeWAV(samples)))
	if err != nil {
		panic(err)
	}
	p, err := ctx.NewPlayer(stream)
	if err != nil {
		panic(err)
	}
	return p
}

func (s *SoundBoard) playVol(p *audio.Player, vol float64) {
	p.SetVolume(clampF(vol, 0, 1))
	p.Rewind()
	p.Play()
}

// 合成器
func chirp(f0, f1, dur, decay float64) []float64 {
	n := int(dur * sampleRate)
	out := make([]float64, n)
	for i := range out {
		t := float64(i) / sampleRate
		f := f0 + (f1-f0)*t/dur
		out[i] = 0.6 * math.Exp(-t*decay) * math.Sin(2*math.Pi*f*t)
	}
	return out
}

func thump() []float64 {
	n := int(math.Floor(0.09 * sampleRate))
	out := make([]float64, n)
	for i := range out {
		t := float64(i) / sampleRate
		env := math.Exp(-t * 70)
		noise := (rand.Float64()*2 - 1) * 0.3
		out[i] = env * (0.8*math.Sin(2*math.Pi*75*t) + noise)
	}
	return out
}

func growl() []float64 {
	n := int(math.Round(0.35 * sampleRate))
	out := make([]float64, n)
	for i := range out {
		t := float64(i) / sampleRate
		env := math.Exp(-t * 6)
		wob := 1 + 0.4*math.Sin(2*math.Pi*3*t) // 频率颤动
		out[i] = 0.35 * env * (saw(110*wob, t) + 0.6*saw(170*wob, t))
	}
	return out
}

func saw(freq, t float64) float64 {
	return 2*math.Mod(freq*t, 1) - 1
}

// ---- 把 PCM 采样编码成 WAV 字节流（内存中完成，无需素材文件）----

func encodeWAV(samples []float64) []byte {
	var buf bytes.Buffer
	dataLen := uint32(len(samples) * 2)
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36)+dataLen)
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // 单声道
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, dataLen)
	for _, s := range samples {
		v := int16(s * 32767)
		if v > 32767 {
			v = 32767
		}
		if v < -32768 {
			v = -32768
		}
		binary.Write(&buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
