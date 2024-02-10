package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"

	"github.com/joho/godotenv"
)

type clientSecret struct {
	Installed struct {
		ClientID                string   `json:"client_id"`
		ProjectID               string   `json:"project_id"`
		AuthUri                 string   `json:"auth_uri"`
		TokenUri                string   `json:"token_uri"`
		AuthProviderX509CertUrl string   `json:"auth_provider_x509_cert_url"`
		ClientSecret            string   `json:"client_secret"`
		RedirectUris            []string `json:"redirect_uris"`
	} `json:"installed"`
}

type oAuth2Credentials struct {
	AccessToken   string  `json:"access_token"`
	ClientID      string  `json:"client_id"`
	ClientSecret  string  `json:"client_secret"`
	RefreshToken  string  `json:"refresh_token"`
	TokenExpiry   string  `json:"token_expiry"`
	TokenURI      string  `json:"token_uri"`
	UserAgent     *string `json:"user_agent"`
	RevokeURI     string  `json:"revoke_uri"`
	IDToken       *string `json:"id_token"`
	IDTokenJWT    *string `json:"id_token_jwt"`
	TokenResponse struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
		TokenType   string `json:"token_type"`
	} `json:"token_response"`
	Scopes       []string `json:"scopes"`
	TokenInfoURI string   `json:"token_info_uri"`
	Invalid      bool     `json:"invalid"`
	Class        string   `json:"_class"`
	Module       string   `json:"_module"`
}

func createClinetSecret() ([]byte, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		return nil, err
	}
	clientData := clientSecret{
		Installed: struct {
			ClientID                string   `json:"client_id"`
			ProjectID               string   `json:"project_id"`
			AuthUri                 string   `json:"auth_uri"`
			TokenUri                string   `json:"token_uri"`
			AuthProviderX509CertUrl string   `json:"auth_provider_x509_cert_url"`
			ClientSecret            string   `json:"client_secret"`
			RedirectUris            []string `json:"redirect_uris"`
		}{
			ClientID:                os.Getenv("YOUTUBE_CLIENT_ID"),
			ProjectID:               os.Getenv("YOUTUBE_PROJECT_ID"),
			AuthUri:                 "https://accounts.google.com/o/oauth2/auth",
			TokenUri:                "https://oauth2.googleapis.com/token",
			AuthProviderX509CertUrl: "https://www.googleapis.com/oauth2/v1/certs",
			ClientSecret:            os.Getenv("YOUTUBE_CLIENT_SECRET"),
			RedirectUris:            []string{"http://localhost"},
		},
	}
	return json.Marshal(clientData)
}

func createOAuth2() ([]byte, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		return nil, err
	}
	oauth2Data := oAuth2Credentials{
		AccessToken:  os.Getenv("YOOUTUBE_ACCESS_TOKEN"),
		ClientID:     os.Getenv("YOUTUBE_CLIENT_ID"),
		ClientSecret: os.Getenv("YOUTUBE_CLIENT_SECRET"),
		RefreshToken: os.Getenv("YOUTUBE_REFRESH_TOKEN"),
		TokenExpiry:  os.Getenv("YOUTUBE_TOKEN_EXPIRY"),
		TokenURI:     "https://oauth2.googleapis.com/token",
		UserAgent:    nil,
		RevokeURI:    "https://oauth2.googleapis.com/revoke",
		IDToken:      nil,
		IDTokenJWT:   nil,
		TokenResponse: struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
			Scope       string `json:"scope"`
			TokenType   string `json:"token_type"`
		}{
			AccessToken: os.Getenv("YOUTUBE_ACCESS_TOKEN"),
			ExpiresIn:   3599,
			Scope:       "https://www.googleapis.com/auth/youtube.upload",
			TokenType:   "Bearer",
		},
		Scopes:       []string{"https://www.googleapis.com/auth/youtube.upload"},
		TokenInfoURI: "https://oauth2.googleapis.com/tokeninfo",
		Invalid:      false,
		Class:        "OAuth2Credentials",
		Module:       "oauth2client.client",
	}
	return json.Marshal(oauth2Data)
}

func getToken() (*oauth2.Token, error) {
	f, err := createOAuth2()
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}
	t := &oauth2.Token{}
	err = json.Unmarshal(f, t)
	return t, err
}

func main() {
	// client_secret.jsonとoauth2.jsonのパスを設定
	b, err := createClinetSecret()
	if err != nil {
		fmt.Printf("Unable to read client secret file: %v", err)
	}

	// OAuth2クライアント作成
	ctx := context.Background()
	config, err := google.ConfigFromJSON(b, youtube.YoutubeUploadScope)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	token, err := getToken()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	client := config.Client(ctx, token)

	// YouTube APIサービス作成
	service, err := youtube.New(client)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// 動画アップロード
	upload := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       "testtitle",
			Description: "testdescription",
			CategoryId:  "22",
		},
		Status: &youtube.VideoStatus{PrivacyStatus: "unlisted"},
	}

	// APIは、tagsが空文字列の場合、400 Bad Requestレスポンスを返す。
	if strings.Trim("golang test", "") != "" {
		upload.Snippet.Tags = strings.Split("golang test", ",")
	}

	call := service.Videos.Insert([]string{"snippet", "status"}, upload)

	file, err := os.Open("gotest.mp4")
	defer file.Close()
	if err != nil {
		log.Fatalf("Error opening %v: %v", "gotest.mp4", err)
	}

	response, err := call.Media(file).Do()
	if err != nil {
		log.Fatalf("Error making YouTube API call: %v", err)
	}
	fmt.Printf("Upload successful! Video ID: %v\n", response.Id)
}

