package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gxp-map-video-backend/internal/gpx"
	"gxp-map-video-backend/internal/route"
)

// API Key 不入库：优先环境变量 LLM_API_KEY，其次本地文件 data/llm-api-key.txt
const defaultLLMKey = ""

const defaultSystemPrompt = `你是一名越野跑赛事解说导演。根据输入的路线分析数据，生成一份"视频演出配置"，用于驱动地图预演视频的镜头参数与解说时间轴。只输出一个合法 JSON 对象，不要任何解释文字或 markdown 代码块。

你必须完成四件事：
1. 镜头调参：根据路线地形特征选择 camera 参数。多陡峭高山 → 更高高度(2400-3600)配合较低俯角(45-55)；平缓开阔 → 较低高度(1200-2000)配较高俯角(55-70)。时长越短(速度越快)应选更高高度。
2. 排布解说事件：在 waypoints（检查点）和 highlights（最高点/最陡段/最长下降）附近选择事件点；事件数量 4-10 个；atKm 必须升序且互不重叠。
3. 撰写解说词：中文口语化，每条 30-120 字（约 8-30 秒语音）。提到该位置的真实地名/检查点名/爬升数据；script 之间内容不得重复；开头一条欢迎/介绍路线，结尾前一条总结。语气符合 hints.style。
4. 时间轴纪律：估算每条解说语音时长约等于 字数/4.5 秒。相邻两条事件按 secPerKm 换算的播放时间间隔必须大于等于 前一条语音时长 + holdAfter + holdBefore，否则必须拉开 atKm 距离。全部解说语音总时长不超过 durationSec 的 30%。

禁止事项：
- 不要修改本说明之外的任何结构；不要新增字段
- 不要使用输入中不存在的地名；数字(爬升/坡度)必须来自输入数据
- 不要输出 emoji；不要输出英文解说

输出 JSON 字段与取值范围（超出范围会被程序裁剪）：
meta: { title: string<=30字, summary: string<=80字 }
playback: { totalDurationSec: int 60-1800, stepMode: "off"|"hill-skip" }
camera: { altitude: int 100-8000, pitch: int 40-80, lookAhead: int 40-120, deadzone: int 10-45, turnRate: 0.1-1.0, curveSlow: { on: bool, strength: 0.2-0.9 }, avoidAhead: bool }
intro: { enabled: bool, duration: int 2-8, startAltitude: int 2000-10000, endAltitude: int 100-8000, startPitch: int 5-85, endPitch: int 5-85, spiralDeg: int 90-360 }
outro: { enabled: bool, duration: int 2-10, endAltitude: int 2000-10000, endPitch: int 15-60, spiralDeg: int 90-360 }
events: [{ atKm: float, type: "COMMENTARY"|"WARNING"|"POI"|"VIEWPOINT"|"JUNCTION"|"REST", title: string<=20字, script: string 30-120字, voice: "zh-CN-XiaoxiaoNeural"|"zh-CN-YunxiNeural"|"zh-CN-YunyangNeural"|"zh-CN-XiaoyiNeural", holdBefore: 0-5, holdAfter: 0-5 }]`

// ShowConfig 演出配置（与前端 applyConfig 消费的结构一致）
type ShowConfig struct {
	Meta     ShowMeta     `json:"meta"`
	Playback ShowPlayback `json:"playback"`
	Camera   ShowCamera   `json:"camera"`
	Intro    ShowIntro    `json:"intro"`
	Outro    ShowOutro    `json:"outro"`
	Events   []ShowEvent  `json:"events"`
}

type ShowMeta struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type ShowPlayback struct {
	TotalDurationSec int    `json:"totalDurationSec"`
	StepMode         string `json:"stepMode"`
}

type ShowCurveSlow struct {
	On       bool    `json:"on"`
	Strength float64 `json:"strength"`
}

type ShowCamera struct {
	Altitude   int            `json:"altitude"`
	Pitch      int            `json:"pitch"`
	LookAhead  int            `json:"lookAhead"`
	Deadzone   int            `json:"deadzone"`
	TurnRate   float64        `json:"turnRate"`
	CurveSlow  ShowCurveSlow  `json:"curveSlow"`
	AvoidAhead bool           `json:"avoidAhead"`
}

type ShowIntro struct {
	Enabled       bool    `json:"enabled"`
	Duration      int     `json:"duration"`
	StartAltitude int     `json:"startAltitude"`
	EndAltitude   int     `json:"endAltitude"`
	StartPitch    int     `json:"startPitch"`
	EndPitch      int     `json:"endPitch"`
	SpiralDeg     int     `json:"spiralDeg"`
}

type ShowOutro struct {
	Enabled     bool    `json:"enabled"`
	Duration    int     `json:"duration"`
	EndAltitude int     `json:"endAltitude"`
	EndPitch    int     `json:"endPitch"`
	SpiralDeg   int     `json:"spiralDeg"`
}

type ShowEvent struct {
	AtKm       float64 `json:"atKm"`
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Script     string  `json:"script"`
	Voice      string  `json:"voice"`
	HoldBefore float64 `json:"holdBefore"`
	HoldAfter  float64 `json:"holdAfter"`
}

var validEventTypes = map[string]bool{
	"COMMENTARY": true, "WARNING": true, "POI": true,
	"VIEWPOINT": true, "JUNCTION": true, "REST": true,
}

var validVoices = map[string]bool{
	"zh-CN-XiaoxiaoNeural": true, "zh-CN-YunxiNeural": true,
	"zh-CN-YunyangNeural": true, "zh-CN-XiaoyiNeural": true,
}

// ValidVoice 检查音色是否在允许列表内
func ValidVoice(v string) bool { return validVoices[v] }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func clampInt(v int, lo, hi int) int {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}

// ParseAndClamp 解析 LLM 输出并按约束钳制所有字段
func ParseAndClamp(content string, totalKm float64) (*ShowConfig, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("输出中未找到 JSON 对象")
	}
	content = content[start : end+1]

	var cfg ShowConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}

	// playback
	if cfg.Playback.TotalDurationSec == 0 { cfg.Playback.TotalDurationSec = 600 }
	cfg.Playback.TotalDurationSec = clampInt(cfg.Playback.TotalDurationSec, 60, 1800)
	if cfg.Playback.StepMode != "hill-skip" { cfg.Playback.StepMode = "off" }

	// camera
	if cfg.Camera.Altitude == 0 { cfg.Camera.Altitude = 1800 }
	cfg.Camera.Altitude = clampInt(cfg.Camera.Altitude, 100, 8000)
	if cfg.Camera.Pitch == 0 { cfg.Camera.Pitch = 60 }
	cfg.Camera.Pitch = clampInt(cfg.Camera.Pitch, 40, 80)
	cfg.Camera.LookAhead = clampInt(cfg.Camera.LookAhead, 40, 120)
	cfg.Camera.Deadzone = clampInt(cfg.Camera.Deadzone, 10, 45)
	cfg.Camera.TurnRate = clamp(cfg.Camera.TurnRate, 0.1, 1.0)
	cfg.Camera.CurveSlow.Strength = clamp(cfg.Camera.CurveSlow.Strength, 0.2, 0.9)

	// intro
	cfg.Intro.Duration = clampInt(cfg.Intro.Duration, 2, 8)
	cfg.Intro.StartAltitude = clampInt(cfg.Intro.StartAltitude, 2000, 10000)
	cfg.Intro.EndAltitude = clampInt(cfg.Intro.EndAltitude, 100, 8000)
	cfg.Intro.StartPitch = clampInt(cfg.Intro.StartPitch, 5, 85)
	cfg.Intro.EndPitch = clampInt(cfg.Intro.EndPitch, 5, 85)
	cfg.Intro.SpiralDeg = clampInt(cfg.Intro.SpiralDeg, 90, 360)

	// outro
	cfg.Outro.Duration = clampInt(cfg.Outro.Duration, 2, 10)
	cfg.Outro.EndAltitude = clampInt(cfg.Outro.EndAltitude, 2000, 10000)
	cfg.Outro.EndPitch = clampInt(cfg.Outro.EndPitch, 15, 60)
	cfg.Outro.SpiralDeg = clampInt(cfg.Outro.SpiralDeg, 90, 360)

	// events
	cleaned := []ShowEvent{}
	for _, e := range cfg.Events {
		if strings.TrimSpace(e.Script) == "" { continue }
		if !validEventTypes[e.Type] { e.Type = "COMMENTARY" }
		if !validVoices[e.Voice] { e.Voice = "zh-CN-XiaoxiaoNeural" }
		e.AtKm = clamp(e.AtKm, 0.2, math.Max(0.5, totalKm-0.5))
		e.Title = truncate(e.Title, 20)
		e.Script = truncate(strings.TrimSpace(e.Script), 160)
		e.HoldBefore = clamp(e.HoldBefore, 0, 5)
		e.HoldAfter = clamp(e.HoldAfter, 0, 5)
		cleaned = append(cleaned, e)
	}
	sort.Slice(cleaned, func(i, j int) bool { return cleaned[i].AtKm < cleaned[j].AtKm })
	// 相邻过近时合并（保留前者，文案拼接）
	merged := []ShowEvent{}
	for _, e := range cleaned {
		if len(merged) > 0 && e.AtKm-merged[len(merged)-1].AtKm < 0.3 {
			merged[len(merged)-1].Script = truncate(merged[len(merged)-1].Script+e.Script, 160)
			continue
		}
		merged = append(merged, e)
	}
	if len(merged) > 40 { merged = merged[:40] }
	if len(merged) == 0 {
		return nil, fmt.Errorf("LLM 未生成任何有效事件")
	}
	cfg.Events = merged

	if strings.TrimSpace(cfg.Meta.Title) == "" { cfg.Meta.Title = "越野路线预演" }
	cfg.Meta.Title = truncate(cfg.Meta.Title, 30)
	cfg.Meta.Summary = truncate(cfg.Meta.Summary, 80)
	return &cfg, nil
}

func truncate(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n { return string(r) }
	return string(r[:n]) + "…"
}

// Client 智谱 GLM（OpenAI 兼容接口）
type Client struct {
	Key   string
	Base  string
	Model string
	HTTP  *http.Client
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// loadAPIKey 优先环境变量 LLM_API_KEY，其次本地文件 data/llm-api-key.txt（不入库）
func loadAPIKey() string {
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		return v
	}
	if b, err := os.ReadFile(filepath.Join("data", "llm-api-key.txt")); err == nil {
		return strings.TrimSpace(string(b))
	}
	return defaultLLMKey
}

func NewClient() *Client {
	// GLM-5.3-Flash 为推理模型（始终思考），完整编排可能需要 1-3 分钟
	return &Client{
		Key:   loadAPIKey(),
		Base:  getenv("LLM_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
		Model: getenv("LLM_MODEL", "glm-5.3-flash"),
		HTTP:  &http.Client{Timeout: 1200 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GLM-5.3 系列为常思考模型：effort=low 可大幅缩短生成时间（结构化 JSON 任务足够）
type thinkingCfg struct {
	Type   string `json:"type"`
	Effort string `json:"effort,omitempty"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
	Thinking    *thinkingCfg  `json:"thinking,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// streamChunk SSE 增量块
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason interface{} `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调用 GLM（SSE 流式接收，避免推理模型长生成导致的整体超时）
func (c *Client) Chat(system, user string) (string, error) {
	start := time.Now()
	body, _ := json.Marshal(chatRequest{
		Model:       c.Model,
		Messages:    []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		Temperature: 0.7,
		Stream:      true,
		Thinking:    &thinkingCfg{Type: "enabled", Effort: "low"},
	})
	log.Printf("[LLM] 开始请求 model=%s system=%d字 user=%d字 effort=low", c.Model, len([]rune(system)), len([]rune(user)))
	req, err := http.NewRequest("POST", c.Base+"/chat/completions", bytes.NewReader(body))
	if err != nil { return "", err }
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.HTTP.Do(req)
	if err != nil { return "", fmt.Errorf("LLM 请求失败: %w", err) }
	defer resp.Body.Close()
	log.Printf("[LLM] 响应头到达 %d，耗时 %.1fs", resp.StatusCode, time.Since(start).Seconds())
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM 返回 %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)
	var sb strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") { continue }
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			if payload == "[DONE]" { break }
			continue
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil { continue }
		if chunk.Error != nil && chunk.Error.Message != "" {
			return "", fmt.Errorf("LLM 错误: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) > 0 {
			sb.WriteString(chunk.Choices[0].Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 LLM 流失败: %w", err)
	}
	log.Printf("[LLM] 流结束，共 %d 字，耗时 %.1fs", sb.Len(), time.Since(start).Seconds())
	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", fmt.Errorf("LLM 未返回内容")
	}
	return out, nil
}

// LoadSystemPrompt 读取可编辑的提示词文件（不存在则用内置默认）
func LoadSystemPrompt(dataDir string) string {
	p := filepath.Join(dataDir, "llm-system-prompt.txt")
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return string(b)
	}
	return defaultSystemPrompt
}

// BuildPayload 汇总路线分析数据作为 LLM 输入
func BuildPayload(r *route.Route, points []route.TrackPoint, segments []route.RouteSegment, waypoints []gpx.Waypoint, hints map[string]any) map[string]any {
	totalKm := r.TotalDistance / 1000
	if totalKm <= 0 { totalKm = 1 }

	// 航点里程：最近轨迹点距离
	wpts := []map[string]any{}
	for _, w := range waypoints {
		best := math.MaxFloat64
		for _, p := range points {
			dLat := (p.Latitude - w.Lat) * 111320
			dLng := (p.Longitude - w.Lng) * 111320 * math.Cos(w.Lat*math.Pi/180)
			d := math.Sqrt(dLat*dLat + dLng*dLng)
			if d < best { best = d }
		}
		wpts = append(wpts, map[string]any{"name": w.Name, "atKm": math.Round(best/1000*100) / 100})
	}

	// 高程剖面：每 2% 里程采样
	prof := []map[string]any{}
	totalM := points[len(points)-1].Distance
	step := totalM / 50
	next := 0.0
	for _, p := range points {
		if p.Distance >= next {
			prof = append(prof, map[string]any{
				"atKm":       math.Round(p.Distance/1000*100) / 100,
				"elevationM": p.Elevation,
				"slope":      p.Slope,
			})
			next += step
		}
	}

	// 亮点
	segs := []map[string]any{}
	highlights := []map[string]any{}
	maxElev := math.MaxFloat64
	var maxPt map[string]any
	var steepest, longestDrop map[string]any
	steepestSlope := math.Inf(-1)
	longestDropM := math.Inf(-1)
	// 分段过多时均匀采样到 ≤60 条（保持顺序），控制 LLM 输入体积
	stepSeg := 1
	if n := len(segments); n > 60 {
		stepSeg = int(math.Ceil(float64(n) / 60))
	}
	for i, s := range segments {
		if i%stepSeg == 0 || i == len(segments)-1 {
			segs = append(segs, map[string]any{
				"startKm":  math.Round(s.StartDistance/1000*100) / 100,
				"endKm":    math.Round(s.EndDistance/1000*100) / 100,
				"type":     s.Type,
				"avgSlope": s.AverageSlope,
				"gainM":    s.ElevationGain,
				"lossM":    s.ElevationLoss,
			})
		}
		if s.AverageSlope > 3 && s.AverageSlope > steepestSlope {
			steepestSlope = s.AverageSlope
			steepest = map[string]any{
				"kind": "steepest_climb", "fromKm": math.Round(s.StartDistance/1000*100) / 100,
				"toKm": math.Round(s.EndDistance/1000*100) / 100, "avgSlope": s.AverageSlope, "gainM": s.ElevationGain,
			}
		}
		if s.AverageSlope < -3 && s.ElevationLoss > longestDropM {
			longestDropM = s.ElevationLoss
			longestDrop = map[string]any{
				"kind": "longest_descent", "fromKm": math.Round(s.StartDistance/1000*100) / 100,
				"toKm": math.Round(s.EndDistance/1000*100) / 100, "dropM": s.ElevationLoss,
			}
		}
	}
	for _, p := range points {
		if maxElev == math.MaxFloat64 || p.Elevation > maxElev {
			maxElev = p.Elevation
			maxPt = map[string]any{"kind": "highest_point", "atKm": math.Round(p.Distance/1000*100) / 100, "elevationM": p.Elevation}
		}
	}
	if maxPt != nil { highlights = append(highlights, maxPt) }
	if steepest != nil { highlights = append(highlights, steepest) }
	if longestDrop != nil { highlights = append(highlights, longestDrop) }

	return map[string]any{
		"route": map[string]any{
			"name":            r.Name,
			"totalDistanceKm": math.Round(totalKm*10) / 10,
			"totalAscentM":    r.TotalAscent,
			"totalDescentM":   r.TotalDescent,
			"minElevationM":   r.MinElevation,
			"maxElevationM":   r.MaxElevation,
			"waypoints":       wpts,
		},
		"highlights":       highlights,
		"elevationProfile": prof,
		"segments":         segs,
		"hints":            hints,
	}
}

// WaypointAt 途经点及其在路线上的里程位置
type WaypointAt struct {
	Name string
	AtKm float64
	Ele  float64
}

// WaypointsAtKm 计算每个航点在轨迹上的最近里程位置
func WaypointsAtKm(waypoints []gpx.Waypoint, points []route.TrackPoint) []WaypointAt {
	out := []WaypointAt{}
	for _, w := range waypoints {
		best := math.MaxFloat64
		var atKm, ele float64
		for _, p := range points {
			dLat := (p.Latitude - w.Lat) * 111320
			dLng := (p.Longitude - w.Lng) * 111320 * math.Cos(w.Lat*math.Pi/180)
			d := math.Sqrt(dLat*dLat + dLng*dLng)
			if d < best {
				best = d
				atKm = p.Distance / 1000
				ele = p.Elevation
			}
		}
		out = append(out, WaypointAt{Name: w.Name, AtKm: math.Round(atKm*100) / 100, Ele: math.Round(ele*10) / 10})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AtKm < out[j].AtKm })
	return out
}

// EnsureWaypointEvents 保证每个途经点都有事件（LLM 遗漏时用真实数据补齐）。
// 返回是否发生了修改。
func EnsureWaypointEvents(cfg *ShowConfig, wps []WaypointAt) bool {
	changed := false
	for _, wp := range wps {
		covered := false
		for _, e := range cfg.Events {
			if math.Abs(e.AtKm-wp.AtKm) < 0.6 {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		ele := ""
		if wp.Ele > 0 {
			ele = fmt.Sprintf("当前海拔 %.0f 米。", wp.Ele)
		}
		script := truncate(fmt.Sprintf("到达%s，已完成 %.1f 公里。%s接下来按路线继续前进，注意节奏与体力分配。", wp.Name, wp.AtKm, ele), 160)
		cfg.Events = append(cfg.Events, ShowEvent{
			AtKm: wp.AtKm, Type: "POI", Title: truncate(wp.Name, 20),
			Script: script, Voice: "zh-CN-XiaoxiaoNeural",
			HoldBefore: 0.5, HoldAfter: 0,
		})
		changed = true
	}
	sort.Slice(cfg.Events, func(i, j int) bool { return cfg.Events[i].AtKm < cfg.Events[j].AtKm })
	return changed
}
