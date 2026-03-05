package aliyun

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TokenManager 管理阿里云 Token
type TokenManager struct {
	accessKeyID     string
	accessKeySecret string
	token           string
	expireTime      time.Time
	mu              sync.RWMutex
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(accessKeyID, accessKeySecret string) *TokenManager {
	return &TokenManager{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
	}
}

// GetToken 获取有效的 Token
func (tm *TokenManager) GetToken() (string, error) {
	tm.mu.RLock()
	if tm.token != "" && time.Now().Before(tm.expireTime.Add(-5*time.Minute)) {
		token := tm.token
		tm.mu.RUnlock()
		return token, nil
	}
	tm.mu.RUnlock()

	return tm.refreshToken()
}

// refreshToken 刷新 Token
func (tm *TokenManager) refreshToken() (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 双重检查
	if tm.token != "" && time.Now().Before(tm.expireTime.Add(-5*time.Minute)) {
		return tm.token, nil
	}

	token, expireTime, err := tm.createToken()
	if err != nil {
		return "", err
	}

	tm.token = token
	tm.expireTime = expireTime

	return token, nil
}

// createToken 调用阿里云 API 创建 Token
func (tm *TokenManager) createToken() (string, time.Time, error) {
	// 公共参数
	params := map[string]string{
		"AccessKeyId":      tm.accessKeyID,
		"Action":           "CreateToken",
		"Format":           "JSON",
		"RegionId":         "cn-shanghai",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   uuid.New().String(),
		"SignatureVersion": "1.0",
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2019-02-28",
	}

	// 计算签名
	signature := tm.sign(params)
	params["Signature"] = signature

	// 构建请求 URL
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}

	reqURL := "https://nls-meta.cn-shanghai.aliyuncs.com/?" + query.Encode()

	resp, err := http.Get(reqURL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Token struct {
			ID         string `json:"Id"`
			ExpireTime int64  `json:"ExpireTime"`
		} `json:"Token"`
		ErrMsg string `json:"ErrMsg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Token.ID == "" {
		return "", time.Time{}, fmt.Errorf("failed to get token: %s", result.ErrMsg)
	}

	expireTime := time.Unix(result.Token.ExpireTime, 0)

	return result.Token.ID, expireTime, nil
}

// sign 计算签名
func (tm *TokenManager) sign(params map[string]string) string {
	// 按 key 排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建待签名字符串
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, percentEncode(k)+"="+percentEncode(params[k]))
	}

	canonicalizedQueryString := strings.Join(pairs, "&")
	stringToSign := "GET&%2F&" + percentEncode(canonicalizedQueryString)

	// HMAC-SHA1
	mac := hmac.New(sha1.New, []byte(tm.accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return signature
}

// percentEncode URL 编码
func percentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}
