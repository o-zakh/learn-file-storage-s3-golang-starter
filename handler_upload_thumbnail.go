package main

import (
	"fmt"
	"io"
	"net/http"

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

	mediaType := header.Header.Get("Content-Type")

	data, err := io.ReadAll(file)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read multipart data", err)
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

	tn := thumbnail{
		data:      data,
		mediaType: mediaType,
	}

	videoThumbnails[videoID] = tn

	newTnURL := fmt.Sprintf("http://localhost:%s/api/thumbnails/%v", cfg.port, videoID)

	dbVideo.ThumbnailURL = &newTnURL

	err = cfg.db.UpdateVideo(dbVideo)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update the video data on the server", err)
		return
	}

	respondWithJSON(w, http.StatusOK, dbVideo)
}
