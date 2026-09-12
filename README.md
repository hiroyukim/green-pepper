# gp

[![CI](https://github.com/hiroyukim/green-pepper/actions/workflows/ci.yml/badge.svg)](https://github.com/hiroyukim/green-pepper/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

軽量なAPIクライアント。ブラウザ上でリクエストを組み立てて送る`gp serve`と、
1つのリクエストテンプレートをCSVの各行で変数展開しながら繰り返し実行する`gp run`の2つの使い方がある。

## Build

```sh
go build -o gp .
```

## Web UI (`gp serve`)

```sh
gp serve [request-file|collection-dir] [--env <env-file>] [--port <port>] [--timeout <duration>]
```

`request-file`は省略可能。指定した場合はその内容を初期値として読み込み、省略した場合は空のリクエスト
（`GET` / URL空）から編集を始められる。ディレクトリを渡した場合はコレクションとして扱う(詳細は後述)。

```sh
gp serve
gp serve examples/request.yaml --env examples/env.yaml --port 8080
```

`http://localhost:8080` を開くと、以下を備えたリクエストビルダーが表示される。

- **Method / URL / Headers / Body編集** — テキストで直接編集でき、`{{var}}`テンプレート変数を書ける
- **Query Params** — URL下のテーブルでクエリパラメータをキー・値で編集でき、URL文字列と自動的に同期する
- **Authorization** — No Auth / Basic Auth / Bearer Token / API Key(Header or Query)を選ぶと、対応する`Authorization`ヘッダー(またはクエリパラメータ)を自動生成する
- **Body種別** — none / raw(JSON、整形ボタン付き) / x-www-form-urlencoded / form-data を切り替えられ、`Content-Type`ヘッダーも自動で付与される
- **cURLインポート/コピー** — cURLコマンドを貼り付けてMethod/URL/Headers/Bodyに展開したり、逆に編集中のリクエストをcURLコマンドとしてコピーしたりできる
- **単発送信** — 「単発送信」ボタンでリクエストを1回実行し、ステータス・ヘッダー・ボディをその場で確認できる。JSONレスポンスはPretty/Raw切り替えとシンタックスハイライト付きで表示され、コピー・ダウンロードもできる
- **CSVで一括実行** — CSVファイルをドラッグ&ドロップ(またはクリック)でアップロードすると、実行前に列名・行数・先頭数行をプレビューし、テンプレート変数がCSV・環境変数のどちらにもカバーされていない場合は警告する。「CSVで実行」を押すと`gp run`と同じ結果が表で表示される
- **環境変数編集・YAMLダウンロード** — 環境変数もブラウザ上で編集でき、編集中のリクエストはYAMLとしてダウンロードして`gp run`にそのまま使い回せる

### コレクション

`request-file`の代わりにディレクトリを渡すと、そのディレクトリ配下の`*.yaml`/`*.yml`ファイル群を
コレクションとして扱える（フラットな一覧のみで、フォルダの入れ子には対応しない)。

```sh
gp serve ./my-collection/
```

画面上部にコレクション内のリクエスト名一覧が表示され、選ぶとそのリクエストが編集画面に読み込まれる。
編集中の内容は「名前を付けて保存」でコレクションディレクトリにYAMLとして保存・上書きできる。
単一ファイルまたは無指定で起動した場合はこれまで通りで、コレクション機能は表示されない。

## CLI (`gp run`)

CSVの各行で変数展開しながらリクエストテンプレートを繰り返し実行し、結果を表で表示する。CIでの疎通確認などに向く。

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

`gp serve`で組み立てたリクエストを「YAMLをダウンロード」で保存すれば、そのまま`<request-file>`として使える。

## Releases

`v*` 形式のタグをpushすると、GitHub Actions ([goreleaser](https://goreleaser.com/)) が
linux/darwin/windows × amd64/arm64 のバイナリをビルドし、GitHub Releaseに自動でアップロードする
([.github/workflows/release.yml](.github/workflows/release.yml))。

ビルド済みバイナリは [Releases](https://github.com/hiroyukim/green-pepper/releases) から取得できる。

## License

[Apache License 2.0](LICENSE)
