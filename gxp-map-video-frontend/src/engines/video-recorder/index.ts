import maplibregl from 'maplibre-gl'

export class VideoRecorder {
  private stream: MediaStream | null = null
  private recorder: MediaRecorder | null = null
  private chunks: Blob[] = []
  private speedMultiplier = 1

  async start(canvas: HTMLCanvasElement | null = null) {
    this.chunks = []
    const sourceCanvas = canvas ?? (document.querySelector('canvas') as HTMLCanvasElement | null)

    if (sourceCanvas) {
      this.stream = sourceCanvas.captureStream(30)
    } else {
      const mapCanvas = document.querySelector('.maplibregl-canvas') as HTMLCanvasElement
      if (mapCanvas) {
        this.stream = mapCanvas.captureStream(30)
      } else {
        throw new Error('No canvas found for recording')
      }
    }

    const mimeType = this.getSupportedMimeType()
    this.recorder = new MediaRecorder(this.stream!, {
      mimeType,
      videoBitsPerSecond: 8_000_000,
    })

    this.recorder.ondataavailable = (e) => {
      if (e.data.size > 0) this.chunks.push(e.data)
    }

    this.recorder.start(1000)
  }

  setSpeed(multiplier: number) {
    this.speedMultiplier = multiplier
  }

  async stop(): Promise<Blob> {
    return new Promise((resolve, reject) => {
      if (!this.recorder) { reject(new Error('Recorder not started')); return }
      this.recorder.onstop = () => {
        const blob = new Blob(this.chunks, { type: this.getSupportedMimeType() })
        resolve(blob)
      }
      this.recorder.stop()
      this.stream?.getTracks().forEach(t => t.stop())
      this.stream = null
      this.recorder = null
    })
  }

  get isRecording() {
    return this.recorder?.state === 'recording'
  }

  private getSupportedMimeType(): string {
    const types = ['video/mp4;codecs=h264', 'video/webm;codecs=vp9', 'video/webm;codecs=vp8', 'video/webm']
    for (const type of types) {
      if (MediaRecorder.isTypeSupported(type)) return type
    }
    return 'video/webm'
  }
}
