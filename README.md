# test-http-server

- 8080/tcp を listen する
  - LISTEN_ADDR で指定可能 (デフォルトが `:8080`)
- Request Header を body で返す
- Request の URL を .json で終わらせると json で返す
- Query String の sleep=time.Duration で指定の時間 sleep する
- Query String の stress=time.Duration で指定の時間 CPU を使いまくる
  - Qeury String の cores=int で使用する core の数を指定する
  - 未指定 or 0 の場合はプロセスから見える core の数
