//go:build ignore

package main

import (
    "fmt"
    "time"
    "bytes"
    "errors"
    "io/ioutil"
    "net/http"
    "encoding/json"
    "encoding/base64"
    "github.com/google/uuid"
)
//TTSServResponse response from backend srvs
type TTSServResponse struct {
    ReqID    string        `json:"reqid"`
    Code      int          `json:"code"`
    Message   string       `json:"Message"`
    Operation string       `json:"operation"`
    Sequence  int          `json:"sequence"`
    Data      string       `json:"data"`
}
func httpPost(url string, headers map[string]string, body []byte,
    timeout time.Duration) ([]byte, error) {
    client := &http.Client{
        Timeout: timeout,
    }
    req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
    if err != nil {
        return nil, err
    }
    for key, value := range headers {
        req.Header.Set(key, value)
    }
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    retBody, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    return retBody, err
}
func synthesis(text string) ([]byte, error) {
    reqID := uuid.NewString()
    params := make(map[string]map[string]interface{})
    params["app"] = make(map[string]interface{})
    //填写平台申请的appid
    params["app"]["appid"] = "xxxx"
    //这部分的token不生效，填写下方的默认值就好
    params["app"]["token"] = "access_token"
    //填写平台上显示的集群名称
    params["app"]["cluster"] = "xxxx"
    params["user"] = make(map[string]interface{})
    //这部分如有需要，可以传递用户真实的ID，方便问题定位
    params["user"]["uid"] = "uid"
    params["audio"] = make(map[string]interface{})
    //填写选中的音色代号
    params["audio"]["voice_type"] = "xxxx"
    params["audio"]["encoding"] = "wav"
    params["audio"]["speed_ratio"] = 1.0
    params["audio"]["volume_ratio"] = 1.0
    params["audio"]["pitch_ratio"] = 1.0
    params["request"] = make(map[string]interface{})
    params["request"]["reqid"] = reqID
    params["request"]["text"] = text
    params["request"]["text_type"] = "plain"
    params["request"]["operation"] = "query"

    headers := make(map[string]string)
    headers["Content-Type"] = "application/json"
    //bearerToken为saas平台对应的接入认证中的Token
    headers["Authorization"] = fmt.Sprintf("Bearer;%s", BearerToken)

    // URL查看上方第四点: 4.并发合成接口(POST)
    url := "https://xxxxxxxx"
    timeo := 30*time.Second
    bodyStr, _ := json.Marshal(params)
    synResp, err := httpPost(url, headers,
        []byte(bodyStr), timeo)
    if err != nil {
        fmt.Printf("http post fail [err:%s]\n", err.Error())
        return nil, err
    }
    fmt.Printf("resp body:%s\n", synResp)
    var respJSON TTSServResponse
    err = json.Unmarshal(synResp, &respJSON)
    if err != nil {
        fmt.Printf("unmarshal response fail [err:%s]\n", err.Error())
        return nil, err
    }
    code := respJSON.Code
    if code != 3000 {
        fmt.Printf("code fail [code:%d]\n", code)
        return nil, errors.New("resp code fail")
    }

    audio, _ := base64.StdEncoding.DecodeString(respJSON.Data)
    return audio, nil
}
func main() {
    text := "字节跳动语音合成"
    audio, err := synthesis(text)
    if err != nil {
        fmt.Printf("synthesis fail [err:%s]\n", err.Error())
        return
    }
    fmt.Printf("get audio succ len[%d]\n", len(audio))
}
