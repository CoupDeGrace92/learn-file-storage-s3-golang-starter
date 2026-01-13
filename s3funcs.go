package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignedClient := s3.NewPresignClient(s3Client)
	params := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	req, err := presignedClient.PresignGetObject(context.Background(), &params, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", nil
	}

	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil || *video.VideoURL == "" {
		return video, nil
	}

	slicedUrl := strings.Split(*video.VideoURL, ",")
	if len(slicedUrl) != 2 {
		return database.Video{}, errors.New("Failed to correctly parse VideoURL from video struct")
	}
	bucket := slicedUrl[0]
	key := slicedUrl[1]

	url, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Minute*3)
	if err != nil {
		return database.Video{}, err
	}

	video.VideoURL = &url
	return video, nil
}
