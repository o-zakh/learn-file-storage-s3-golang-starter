package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	maxSize := 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxSize))

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

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Wrong video ID", err)
		return
	}
	if userID != dbVideo.CreateVideoParams.UserID {
		respondWithError(w, http.StatusUnauthorized, "Couldn't access the video", nil)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't process media type properties", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media format", err)
		return
	}

	videoFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save file", err)
		return
	}
	defer os.Remove(videoFile.Name())
	defer videoFile.Close()

	_, err = io.Copy(videoFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could't copy videofile contents", err)
		return
	}

	_, err = videoFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't reset the temp videofile offset", err)
		return
	}

	src := make([]byte, 32)
	_, err = rand.Read(src)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate random filename", err)
		return
	}
	filenameBase := hex.EncodeToString(src)

	_, mediaFormat, ok := strings.Cut(mediaType, "/")
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Couldn't extract media format", err)
		return
	}

	fileKey := fmt.Sprintf("%s.%s", filenameBase, mediaFormat)

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        videoFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't load the videofile on the server", err)
		return
	}

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileKey)

	dbVideo.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(dbVideo)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update the videoURL in the database", err)
		return
	}
	respondWithJSON(w, http.StatusOK, struct{}{})
}
