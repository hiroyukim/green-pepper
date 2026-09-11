# gp

[![CI](https://github.com/hiroyukim/green-pepper/actions/workflows/ci.yml/badge.svg)](https://github.com/hiroyukim/green-pepper/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

軽量なCLI APIクライアント。
1つのリクエストテンプレートをCSVの各行で変数展開しながら繰り返し実行し、結果を表で表示する。

## Build

```sh
go build -o gp .
```

## Usage

```sh
gp run <request-file> [--data <csv-file>] [--env <env-file>] [--timeout <duration>] [--format table|json]
```

- `<request-file>`: リクエストテンプレート (YAML)。`method` / `url` / `headers` / `body` に `{{var}}` を書ける。
- `--data`: CSVファイル。1行目がヘッダー（変数名）、以降の各行が1リクエスト分の変数値。省略時は1回だけ実行する。
- `--env`: デフォルト変数を定義するYAML (`base_url` など)。CSVの値がある場合はそちらが優先される。
- `--timeout`: リクエストごとのタイムアウト（デフォルト30秒）。
- `--format`: 出力形式。`table`（デフォルト、人間向けの表）か `json`（各リクエストの結果を構造化JSONの配列で出力。AIエージェントなどからの利用向け）。

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

## Web UI

CSVファイルをブラウザからアップロードして実行したい場合は `gp serve` を使う。
リクエストテンプレートと環境変数はコマンドの起動時に指定し、ブラウザ側ではCSVのアップロードと実行だけを行う。

```sh
gp serve <request-file> [--env <env-file>] [--port <port>] [--timeout <duration>]
```

```sh
gp serve examples/request.yaml --env examples/env.yaml --port 8080
```

`http://localhost:8080` を開き、CSVファイルを選んで「実行」を押すと、`gp run` と同じ結果が表で表示される。

## Releases

`v*` 形式のタグをpushすると、GitHub Actions ([goreleaser](https://goreleaser.com/)) が
linux/darwin/windows × amd64/arm64 のバイナリをビルドし、GitHub Releaseに自動でアップロードする
([.github/workflows/release.yml](.github/workflows/release.yml))。

ビルド済みバイナリは [Releases](https://github.com/hiroyukim/green-pepper/releases) から取得できる。

## License

[Apache License 2.0](LICENSE)
