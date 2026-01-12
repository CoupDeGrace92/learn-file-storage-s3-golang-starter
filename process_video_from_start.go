package main

import (
	"os/exec"
)

func processVideoForFastStart(filePath string) (string, error) {
	outPath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outPath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outPath, nil
}
