package drive

import (
	"os"
	"os/exec"
	"path/filepath"
)

// CommandRunner abstracts os/exec.Command for testing.
type CommandRunner interface {
	// Run executes the command and returns its combined output.
	Run(name string, args ...string) ([]byte, error)
}

// ExecCommandRunner is the default CommandRunner that uses os/exec.
type ExecCommandRunner struct{}

func (r *ExecCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// FFmpegVideoProcessor extracts thumbnails from video files using FFmpeg.
type FFmpegVideoProcessor struct {
	runner  CommandRunner
	imgProc ImageProcessor
}

// NewFFmpegVideoProcessor creates a new FFmpegVideoProcessor. imgProc is
// used to convert the extracted PNG frame to WebP. runner が nil の場合は
// ExecCommandRunner を使用する。
func NewFFmpegVideoProcessor(imgProc ImageProcessor, runner CommandRunner) *FFmpegVideoProcessor {
	if runner == nil {
		runner = &ExecCommandRunner{}
	}
	return &FFmpegVideoProcessor{runner: runner, imgProc: imgProc}
}

// osMkdirTemp / osWriteFile はテスト時に差し替え可能。
var osMkdirTemp = os.MkdirTemp
var osWriteFile = os.WriteFile

// GenerateThumbnail extracts a frame at 5% of the video duration and
// converts it to a WebP thumbnail. FFmpeg がない場合や失敗時は nil を返す。
func (p *FFmpegVideoProcessor) GenerateThumbnail(body []byte, mimeType string) (*ProcessedImage, error) {
	if !isMimeVideo(mimeType) {
		return nil, nil
	}

	tmpDir, err := osMkdirTemp("", "misskey-video-*")
	if err != nil {
		return nil, nil
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input")
	outputPath := filepath.Join(tmpDir, "out.png")

	if err := osWriteFile(inputPath, body, 0o600); err != nil {
		return nil, nil
	}

	// FFmpeg: 5% 地点のスクリーンショットを PNG で出力
	_, err = p.runner.Run("ffmpeg",
		"-i", inputPath,
		"-ss", "5%",
		"-vframes", "1",
		"-f", "image2",
		outputPath,
	)
	if err != nil {
		// FFmpeg が見つからない、またはエラーの場合はサムネイルなし
		return nil, nil
	}

	pngData, err := os.ReadFile(outputPath)
	if err != nil || len(pngData) == 0 {
		return nil, nil
	}

	// PNG を ImageProcessor でサムネイルサイズの WebP に変換
	if p.imgProc != nil {
		return p.imgProc.GenerateThumbnail(pngData, "image/png")
	}

	return nil, nil
}
