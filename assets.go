package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
)

type videoDataStream struct {
	Streams []struct {
		Width  int `json:"width,omitempty"`
		Height int `json:"height,omitempty"`
	} `json:"streams"`
}

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	buffer := bytes.Buffer{}
	cmd.Stdout = &buffer

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	videoInfo := videoDataStream{}

	dec := json.NewDecoder(&buffer)
	err = dec.Decode(&videoInfo)
	if err != nil {
		return "", err
	}

	if len(videoInfo.Streams) == 0 {
		return "", fmt.Errorf("videoInfo is empty")
	}

	w := videoInfo.Streams[0].Width
	h := videoInfo.Streams[0].Height

	ratio := float64(w) / float64(h)
	targetHorizontal := 16.0 / 9.0
	targetVertical := 9.0 / 16.0

	if math.Abs(ratio-targetHorizontal) < 0.02 {
		return "16:9", nil
	} else if math.Abs(ratio-targetVertical) < 0.02 {
		return "9:16", nil
	}

	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	newFilePath := fmt.Sprintf("%s.processing", filePath)

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", newFilePath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s", stderr.String())
	}
	return newFilePath, nil
}
