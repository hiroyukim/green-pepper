# gp

PostmanのCollection Runner相当の機能を持つ、軽量なCLI APIクライアント。
1つのリクエストテンプレートをCSVの各行で変数展開しながら繰り返し実行し、結果を表で表示する。

## Build

```sh
go build -o gp .
```

## Usage

```sh
gp run <request-file> [--data <csv-file>] [--env <env-file>] [--timeout <duration>]
```

- `<request-file>`: リクエストテンプレート (YAML)。`method` / `url` / `headers` / `body` に `{{var}}` を書ける。
- `--data`: CSVファイル。1行目がヘッダー（変数名）、以降の各行が1リクエスト分の変数値。省略時は1回だけ実行する。
- `--env`: デフォルト変数を定義するYAML (`base_url` など)。CSVの値がある場合はそちらが優先される。
- `--timeout`: リクエストごとのタイムアウト（デフォルト30秒）。

終了コードは、全リクエストが2xxなら0、1件でも失敗すれば1。CIでの合否判定に使える。

### Example

`examples/request.yaml`:

```yaml
method: GET
url: "{{base_url}}/users/{{id}}"
headers:
  Accept: application/json
```

`examples/env.yaml`:

```yaml
base_url: https://jsonplaceholder.typicode.com
```

`examples/users.csv`:

```csv
id
1
2
9999
```

```sh
gp run examples/request.yaml --data examples/users.csv --env examples/env.yaml
```

```
#  STATUS         TIME  SIZE  id    ERROR
1  200 OK         63ms  509   1
2  200 OK         27ms  509   2
3  404 Not Found  15ms  2     9999

2/3 passed
```
