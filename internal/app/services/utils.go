package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"getytstatsapi/internal/app/models"
)

func hashMap(data []models.VideoInfo) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	hash.Write(jsonData)

	hashString := hex.EncodeToString(hash.Sum(nil))
	return hashString, nil
}
