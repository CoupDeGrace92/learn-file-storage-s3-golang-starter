package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)

	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't retrieve video from db", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User not authorized to make that request", err)
		return
	}
	//We use a signed video so the client can upload to a private bucket:
	video, err = cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error issuing presigned video object", err)
		return
	}

	vidFile, vidHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not retrieve video from request", err)
		return
	}
	defer vidFile.Close()

	mediaType, _, err := mime.ParseMediaType(vidHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error inducing extension type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Media type not supported for videos", err)
		return
	}

	videoTemp, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temp video file", err)
		return
	}
	defer os.Remove(videoTemp.Name())
	defer videoTemp.Close()

	_, err = io.Copy(videoTemp, vidFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying video to temp file", err)
		return
	}

	//We need to reset the tempFile's file pointer to the beginning with .Seek(0, io.SeekStart)
	//In this case, I am not sure why the file pointer moved from the beggining object?
	_, err = videoTemp.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error resetting io.Reader buffer to 0", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(videoTemp.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting aspect ratio of video", err)
		return
	}

	processVidPath, err := processVideoForFastStart(videoTemp.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error moving moov atom to start of file", err)
		return
	}
	defer os.Remove(processVidPath)

	f, err := os.Open(processVidPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening faststart mp4", err)
		return
	}

	defer f.Close()

	b := make([]byte, 32)
	rand.Read(b)
	id := fmt.Sprintf("%s/%s.mp4", aspectRatio, base64.RawURLEncoding.EncodeToString(b))

	s3Params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &id,
		Body:        f,
		ContentType: &mediaType,
	}

	cfg.s3Client.PutObject(r.Context(), &s3Params)

	urlPath := fmt.Sprintf("%s,%s", cfg.s3Bucket, id)
	video.VideoURL = &urlPath

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating internal db with new video", err)
		return
	}
}
