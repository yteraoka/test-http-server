# test-http-server

- 8080/tcp を listen する
  - LISTEN_ADDR, PORT で指定可能 (デフォルトが `0.0.0.0:8080`)
- Request Header を body で返す
- Request の URL を .json で終わらせると json で返す
- Query String の sleep=time.Duration で指定の時間 sleep する
- Query String の stress=time.Duration で指定の時間 CPU を使いまくる
  - Qeury String の cores=int で使用する core の数を指定する
  - 未指定 or 0 の場合はプロセスから見える core の数
- リクエストごとに uuid を生成して response に入れる
- サーバー側の timestamp を response に入れる
  - sleep させた場合、sleep 後の時刻
- `/stream` で sleep しながら chunked response を返す

## container image

```
docker pull ghcr.io/yteraoka/test-http-server:0.1.0
```

## 実行例

```
$ curl -s "http://localhost:8080/"

[Request]
Method: GET
Host: localhost:8080
RequestURI: /
Proto: HTTP/1.1
Content-Length: 0
Close: false
RemoteAddr: 127.0.0.1:60166

[Received Headers]
Accept: */*
User-Agent: curl/7.84.0

[Server Generated]
uuid: 1f7af804-c7ef-421e-88c0-4d9ce2cb8f0c
time: 2022-08-14 20:27:51.662474 +0900 JST m=+9.493387384
```

```
$ curl -s "http://localhost:8080/a.json" | jq .
{
  "generated": {
    "time": "2022-08-14 20:28:04.423643 +0900 JST m=+22.254587266",
    "uuid": "df58179b-417d-4aa9-a5a7-dabc70c728da"
  },
  "headers": {
    "Accept": [
      "*/*"
    ],
    "User-Agent": [
      "curl/7.84.0"
    ]
  },
  "request": {
    "close": false,
    "content-length": 0,
    "method": "GET",
    "proto": "HTTP/1.1",
    "remote-addr": "127.0.0.1:60168",
    "uri": "/a.json"
  }
}
```

```
$ curl -sw "%{time_total}\n" -o /dev/null "http://localhost:8080/?sleep=5s"                                      [main]
5.005371
```

```
$ curl -sw "%{http_code}\n" -o /dev/null "http://localhost:8080/?status=404"                                     [main]
404
```

```
$ curl -sw "%{time_total}\n" -o /dev/null "http://localhost:8080/?stress=10s&cores=2"                            [main]
10.005560
```
