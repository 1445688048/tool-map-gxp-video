# 迭代开发文档

本文档目录包含三个迭代的详细开发文档。

## 迭代概览

| 迭代 | 名称 | 核心目标 | 前置条件 | 影响范围 |
|------|------|----------|----------|----------|
| 1 | GPX → 3D Terrain → 路线跟随播放 | 完成核心播放链路 | 无 | 所有后续迭代的基础 |
| 2 | 路线分析 → 分段 → Story Timeline → Intro/Outro | 路线理解 + 叙事能力 | 迭代 1 完成 | 迭代 3 的 Timeline 数据基础 |
| 3 | 视频导出 → Tile Preload → 性能优化 | MP4 导出 + 长路线支持 | 迭代 1 + 2 完成 | 产品最终交付 |

## 关键决策记录

### 技术栈
- 后端：Go + Gin + GORM + SQLite（modernc.org/sqlite，纯 Go 无 CGo）
- 前端：Vue 3 + TypeScript + Vite + MapLibre GL JS（npm 包）
- 瓦片缓存：Go 后端代理 + 磁盘缓存
- 视频导出：前端 MediaRecorder 录制 WebM → 后端 FFmpeg 转 MP4
- 前后端独立仓库，开发时 Vite proxy 联调

### 分段阈值
- 越野跑：FLAT ±3% / CLIMB +3~+8% / STEEP >+8% / 最小段 200m
- 徒步登山：FLAT ±5% / CLIMB +5~+12% / STEEP >+12% / 最小段 500m
- 山地骑行：FLAT ±4% / CLIMB +4~+10% / STEEP >+10% / 最小段 300m
- 支持自定义

### 步幅跳跃
- 迭代 1 只做实时速度播放
- 迭代 2 预留接口，实现压缩比模式
- 原理：采样关键点 → 两点间线性插值 → 相机始终平滑跟随

### TTS
- 接口抽象化（TTSProvider interface）
- 第一版 MockTTSProvider（按字数估算 duration）
- 后续接 MOSS-TTS-Nano

### AI 解说
- 第一版：规则模板生成 + 用户手动编辑
- 后续：接入 OpenAI/通义 API

## 文档索引

- [迭代 1：GPX → 3D Terrain → 路线跟随播放](./iteration-01-gpx-3d-playback.md)
- [迭代 2：路线分析 → 分段 → Story Timeline → Intro/Outro](./iteration-02-segment-timeline-story.md)
- [迭代 3：视频导出 → Tile Preload → 性能优化](./iteration-03-video-export-preload.md)
