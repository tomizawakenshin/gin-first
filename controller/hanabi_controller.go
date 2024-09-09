package controller

import (
	"context"
	"fmt"
	"gin-fleamarket/dto"
	"gin-fleamarket/models"
	"gin-fleamarket/services"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/google/uuid"

	"cloud.google.com/go/storage"
	"github.com/gin-gonic/gin"
)

type IHanabiController interface {
	Create(ctx *gin.Context)
}

type HanabiController struct {
	services services.IHanabiService
}

func NewHanabiController(service services.IHanabiService) IHanabiController {
	return &HanabiController{services: service}
}

func (c *HanabiController) Create(ctx *gin.Context) {
	user, exists := ctx.Get("user")
	if !exists {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		fmt.Println("認証失敗！")
		return
	}

	// ファイルを取得
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "File upload error"})
		return
	}
	defer file.Close()

	// 一意なファイル名を生成
	objectName := uuid.New().String() + "_" + header.Filename

	// Google Cloud Storage にアップロード
	bucketName := "team17_sokuseki"
	uploadedFileURL, err := uploadFileToGCS(bucketName, objectName, file)
	if err != nil {
		log.Printf("Failed to upload file: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// フォームデータを受け取る
	var input dto.CreateHanabiInput
	input.Name = ctx.PostForm("name")
	input.Description = ctx.PostForm("description")
	input.Tag = ctx.PostForm("tag")
	input.PhotoURL = uploadedFileURL

	// データのバリデーションを手動で行う
	if input.Name == "" || input.Description == "" || input.Tag == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	// Hanabiの作成処理
	userId := user.(*models.User).ID
	newHanabi, err := c.services.Create(input, userId)
	if err != nil {
		log.Printf("Failed to create Hanabi: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// ユーザー情報をプリロード
	if err := c.services.PreloadUser(newHanabi); err != nil {
		log.Printf("Failed to preload user data: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"data": newHanabi})
}

// Google Cloud Storage にファイルをアップロードする関数
func uploadFileToGCS(bucketName, objectName string, file multipart.File) (string, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	// バケットを指定してファイルをアップロード
	bucket := client.Bucket(bucketName)
	wc := bucket.Object(objectName).NewWriter(ctx)
	if _, err = io.Copy(wc, file); err != nil {
		return "", err
	}
	if err := wc.Close(); err != nil {
		return "", err
	}

	// 公開URLを作成
	uploadedFileURL := fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucketName, objectName)
	return uploadedFileURL, nil
}