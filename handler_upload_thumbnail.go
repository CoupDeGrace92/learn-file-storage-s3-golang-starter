package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// ------------------------  AUTH ABOVE ------ THUMB STORAGE BELOW --------------------------------------------//
	const maxMemory = 10 << 20 //this gives us 10mbs since bit shifting 10 << 20 is the same as 10 * 1024 * 1024
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse multipart form", err)
		return
	}
	File, multiHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't form file", err)
		return
	}
	defer File.Close()

	fileType := multiHeader.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(fileType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error inducing extension type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Media type not supported for thumbnails", errors.New("Incompatable media type"))
		return
	}

	extensions, err := mime.ExtensionsByType(fileType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not induce extension type", err)
		return
	}
	if len(extensions) == 0 {
		respondWithError(w, http.StatusBadRequest, "Could not induce extension type", errors.New("No extensions found"))
		return
	}

	b := make([]byte, 32)
	rand.Read(b)
	id := base64.RawURLEncoding.EncodeToString(b)

	extensionString := fmt.Sprintf("%s.%s", id, extensions[0])
	filePath := filepath.Join(cfg.assetsRoot, extensionString)
	outFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create filepath", err)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, File)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing to outfile", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving video from databse", err)
		return
	}
	if video.UserID != userID {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	serverPath := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, extensionString)
	video.ThumbnailURL = &serverPath

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating internal db with new thumbnail", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
