package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't process media type properties", err)
		return
	}

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid media format", err)
		return
	}

	_, mediaFormat, ok := strings.Cut(mediaType, "/")

	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Couldn't extract media format", err)
		return
	}

	randKey := make([]byte, 32)
	rand.Read(randKey)

	filenameBase := base64.RawURLEncoding.EncodeToString(randKey)

	filename := fmt.Sprintf("%s.%s", filenameBase, mediaFormat)

	fp := filepath.Join(cfg.assetsRoot, filename)

	mediaFile, err := os.Create(fp)

	_, err = io.Copy(mediaFile, file)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write media data to the server storage", err)
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't load the video data from the server", err)
		return
	}

	if userID != dbVideo.CreateVideoParams.UserID {
		respondWithError(w, http.StatusUnauthorized, "Access is not granted", nil)
		return
	}

	newTnURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, filename)

	dbVideo.ThumbnailURL = &newTnURL

	err = cfg.db.UpdateVideo(dbVideo)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update the video data on the server", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
