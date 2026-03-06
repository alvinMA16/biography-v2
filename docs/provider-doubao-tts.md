此文档主要是说明 TTS HTTP 接口如何调用。
<span id="_1-接口说明"></span>
# 1. 接口说明
> 接口地址为 **https://openspeech.bytedance.com/api/v1/tts**

<span id="_2-身份认证"></span>
# 2. 身份认证
认证方式采用 Bearer Token.
1)需要在请求的 Header 中填入"Authorization":"Bearer;${token}"
:::warning
Bearer和token使用分号 ; 分隔，替换时请勿保留${}
:::
AppID/Token/Cluster 等信息可参考 [控制台使用FAQ-Q1](/docs/6561/196768#q1：哪里可以获取到以下参数appid，cluster，token，authorization-type，secret-key-？)
<span id="_3-请求方式"></span>
# 3. 请求方式
<span id="_3-1-请求参数"></span>
## 3.1 请求参数
参考文档：[参数基本说明](/docs/6561/79823)
<span id="_3-2-返回参数"></span>
## 3.2 返回参数
参考文档：[参数基本说明](/docs/6561/79823)
<span id="_4-注意事项"></span>
# 4. 注意事项

* 使用 HTTP Post 方式进行请求，返回的结果为 JSON 格式，需要进行解析
* 因 json 格式无法直接携带二进制音频，音频经base64编码。使用base64解码后，即为二进制音频
* 每次合成时 reqid 这个参数需要重新设置，且要保证唯一性（建议使用 UUID/GUID 等生成）

<span id="_5-demo"></span>
# 5. Demo
<span id="python"></span>
### Python
<Attachment link="https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_a24e9f8b99a6d19e3050fd8151919e8a.py" name="tts_http_demo.py" size="1.33KB"></Attachment>
<span id="java"></span>
### Java
<Attachment link="https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_eb8a4d1920e352a3bb15a3e1d8b0638b.zip" name="tts_http_demo.zip" size="13.27KB"></Attachment>
<span id="go"></span>
### Go
<Attachment link="https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_875974e0ad7f3e56db5c8240d7fbedc8.go" name="tts_http_demo.go" size="3.44KB"></Attachment>

