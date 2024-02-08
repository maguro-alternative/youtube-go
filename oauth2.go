package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// この変数は、スクリプトがウェブサーバーを起動して認証フローを開始するか、
// ターミナルウィンドウにURLを表示するかを示します。この設定に基づいて以下の
// インストラクションに注意してください：
// * launchWebServer = true
//   1. ウェブアプリケーション向けのOAuth2資格情報を使用します
//   2. Google APIコンソールで資格情報のための承認済みリダイレクトURIを定義し、
//      configオブジェクトのRedirectURLプロパティをそれらのリダイレクトURIの1つに
//      設定します。例えば：
//      config.RedirectURL = "http://localhost:8090"
//   3. 以下のstartWebServer関数内で、この行のURLを更新して、選択したリダイレクトURIに
//      一致するようにします：
//      listener, err := net.Listen("tcp", "localhost:8090")
//      リダイレクトURIは、ユーザーが認証フローを完了した後に送信されるURIを識別します。
//      リスナーはその後、URL内の認証コードをキャプチャし、このスクリプトに返します。

// * launchWebServer = false
//   1. インストール済みアプリケーション向けのOAuth2資格情報を使用します。
//      (OAuth2クライアントIDのアプリケーションタイプを選択する際に、「その他」を選択します。)
//   2. リダイレクトURIを次のように設定します：
//      config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"
//   3. スクリプトを実行する際に、認証フローを完了します。その後、ブラウザから
//      認証コードをコピーし、コマンドラインに入力します。

const launchWebServer = false

const missingClientSecretsMessage = `
Please configure OAuth 2.0
To make this sample run, you need to populate the client_secrets.json file
found at:
   %v
with information from the {{ Google Cloud Console }}
{{ https://cloud.google.com/console }}
For more information about the client_secrets.json file format, please visit:
https://developers.google.com/api-client-library/python/guide/aaa_client_secrets
`

// getClient は、コンテキストとコンフィグを使用してトークンを取得します。
// 次にクライアントを生成します。生成されたクライアントを返します。
func getClient(scope string) *http.Client {
	ctx := context.Background()

	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// スコープを変更する場合は、以前に保存した認証情報を削除してください。
	// at ~/.credentials/youtube-go.json
	config, err := google.ConfigFromJSON(b, scope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	// ウェブアプリにはこのようなリダイレクトURIを使います。
	// リダイレクト URI は、OAuth2 認証情報に対して有効なものでなければなりません。
	config.RedirectURL = "http://localhost:8090"
	// oauth2.goでlaunchWebServer=falseの場合、以下のリダイレクトURIを使用する。
	// config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"

	cacheFile, err := tokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := tokenFromFile(cacheFile)
	if err != nil {
		authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		if launchWebServer {
			fmt.Println("Trying to get token from web")
			tok, err = getTokenFromWeb(config, authURL)
		} else {
			fmt.Println("Trying to get token from prompt")
			tok, err = getTokenFromPrompt(config, authURL)
		}
		if err == nil {
			saveToken(cacheFile, tok)
		}
	}
	return config.Client(ctx, tok)
}

// startWebServerは、http://localhost:8080でリッスンするウェブサーバーを起動します。
// ウェブサーバーは、3段階の認証フローでのOAuthコードを待機します。
func startWebServer() (codeCh chan string, err error) {
	listener, err := net.Listen("tcp", "localhost:8090")
	if err != nil {
		return nil, err
	}
	codeCh = make(chan string)

	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		codeCh <- code // send code to OAuth flow
		listener.Close()
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Received code: %v\r\nYou can now safely close this browser window.", code)
	}))

	return codeCh, nil
}

// openURLは指定された場所にブラウザウィンドウを開きます。
// このコードは元々以下に表示されました：
//
//	http://stackoverflow.com/questions/10377243/how-can-i-launch-a-process-that-is-not-a-file-in-go
func openURL(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost:4001/").Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("Cannot open URL %s on this platform", url)
	}
	return err
}

// 認証コードをアクセストークンと交換する
func exchangeToken(config *oauth2.Config, code string) (*oauth2.Token, error) {
	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token %v", err)
	}
	return tok, nil
}

// getTokenFromPromptはConfigを使用してTokenをリクエストし、ユーザーに対してコマンドラインでトークンを入力するよう促します。
// 取得されたTokenが戻り値になります。
func getTokenFromPrompt(config *oauth2.Config, authURL string) (*oauth2.Token, error) {
	var code string
	fmt.Printf("Go to the following link in your browser. After completing "+
		"the authorization flow, enter the authorization code on the command "+
		"line: \n%v\n", authURL)

	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}
	fmt.Println(authURL)
	return exchangeToken(config, code)
}

// getTokenFromWebはConfigを使用してTokenをリクエストします。
// 取得されたTokenが戻り値になります。
func getTokenFromWeb(config *oauth2.Config, authURL string) (*oauth2.Token, error) {
	codeCh, err := startWebServer()
	if err != nil {
		fmt.Printf("Unable to start a web server.")
		return nil, err
	}

	err = openURL(authURL)
	if err != nil {
		log.Fatalf("Unable to open authorization URL in web server: %v", err)
	} else {
		fmt.Println("Your browser has been opened to an authorization URL.",
			"This program will resume once authorization has been provided.")
		fmt.Println(authURL)
	}

	// ウェブサーバーがコードを取得するのを待ちます。
	code := <-codeCh
	return exchangeToken(config, code)
}

// tokenCacheFile は、クレデンシャル・ファイルのパス/ファイル名を生成します。
// 生成されたクレデンシャル・パス/ファイル名を返します。
func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape("youtube-go.json")), err
}

// tokenFromFile は指定されたファイル・パスからトークンを取得します。
// 取得したトークンと、発生した読み取りエラーを返します。
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

// saveTokenはファイル・パスを使用してファイルを作成し、トークンをその中に格納します。
func saveToken(file string, token *oauth2.Token) {
	fmt.Println("trying to save token")
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
