package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"

	"github.com/bytedance/sonic"
	ark "github.com/sashabaranov/go-openai"
)

const doubaoApiUrl = "https://ark.cn-beijing.volces.com/api/v3"

// 转换参数，将MultiContent转成一条Content
func ChangeMessages(r *http.Request) error {
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		return errors.New("failed to read request body")
	}

	var openaiReq ark.ChatCompletionRequest
	if err := sonic.Unmarshal(reqBody, &openaiReq); err != nil {
		log.Printf("Error parsing request body: %v", err)
		return errors.New("failed to parse request body")
	}

	// 遍历Messages，将MultiContent转换为Content
	for i := range openaiReq.Messages {
		if len(openaiReq.Messages[i].MultiContent) > 1 {
			for j := range openaiReq.Messages[i].MultiContent {
				if openaiReq.Messages[i].MultiContent[j].Type == ark.ChatMessagePartTypeText {
					openaiReq.Messages[i].Content += openaiReq.Messages[i].MultiContent[j].Text
				}
			}
			// 清空MultiContent
			openaiReq.Messages[i].MultiContent = nil
		}
	}

	// 将转换后的参数重新赋值给请求体
	reqBody, err = sonic.Marshal(openaiReq)
	if err != nil {
		log.Printf("Error marshalling request body: %v", err)
		return errors.New("failed to marshal request body")
	}

	// 将请求体设置为新的bytes.Buffer
	r.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	r.ContentLength = int64(len(reqBody))
	return nil
}

func main() {
	port := 80 // 默认端口号
	// 获取命令行参数
	args := os.Args
	// 检查是否提供了参数
	if len(args) == 2 { // 端口号
		portTmp, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatal(err)
		}
		port = portTmp
	}

	// 定义后端服务器的URL
	backendURL, err := url.Parse(doubaoApiUrl)
	if err != nil {
		log.Fatal(err)
	}

	// 创建反向代理处理器
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	// 创建HTTP服务器
	http.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		// 修改请求的Host头，使其指向后端服务器
		r.Host = backendURL.Host
		if err := ChangeMessages(r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// 使用反向代理处理器处理请求
		proxy.ServeHTTP(w, r)
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
