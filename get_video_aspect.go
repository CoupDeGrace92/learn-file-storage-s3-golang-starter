package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	type Stream struct {
		Index  int `json:"index"`
		Width  int `json:"width"`
		Height int `json:"height"`
	}

	type Videos struct {
		Streams []Stream `json:"streams"`
	}

	var s Videos

	err = json.Unmarshal(out.Bytes(), &s)
	if err != nil {
		return "", err
	}

	strAspect := "other"
	if len(s.Streams) == 0 {
		return "", errors.New("No streams present")
	}
	h, w := s.Streams[0].Height, s.Streams[0].Width
	if h == 0 || w == 0 {
		err = errors.New("Can't have zero height or width")
		return "", err
	}
	aspRatio := float64(w) / float64(h)
	if aspRatio > 1.7 && aspRatio < 1.8 {
		strAspect = "landscape"
	}
	if aspRatio > .5 && aspRatio < .6 {
		strAspect = "portrait"
	}

	return strAspect, nil
}
