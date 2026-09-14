# gp

[![CI](https://github.com/hiroyukim/green-pepper/actions/workflows/ci.yml/badge.svg)](https://github.com/hiroyukim/green-pepper/actions/workflows/ci.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

ブラウザでAPIリクエストを組み立てて送れる、軽量なAPIクライアント。普段の1回きりのリクエストも、
CSVを使った大量データでの繰り返し実行も、同じ画面から行える。コマンドラインからも同じことができるので、
CIに組み込んで自動テストの一部として使うこともできる。

## 目次

- [インストール](#インストール)
  - [ビルド済みバイナリを使う](#ビルド済みバイナリを使う)
  - [ソースからビルドする](#ソースからビルドする)
- [3分でわかる`gp serve`](#3分でわかるgp-serve)
- [Web UI (`gp serve`) — 詳しいリファレンス](#web-ui-gp-serve--詳しいリファレンス)
  - [複数環境を切り替える(例)](#複数環境を切り替える例)
  - [送信履歴(例)](#送信履歴例)
  - [コレクションで複数リクエストを連続実行(例)](#コレクションで複数リクエストを連続実行例)
  - [進捗表示・CSV実行履歴・行ごとの詳細(例)](#進捗表示csv実行履歴行ごとの詳細例)
  - [JSON API (`/api/send`, `/api/run`)](#json-api-apisend-apirun)
- [CLI (`gp run`)](#cli-gp-run)
  - [Example](#example)
- [テストスクリプト (`test_script`)](#テストスクリプト-test_script)
  - [デバッグ出力 (`console.log`)](#デバッグ出力-consolelog)
- [License](#license)

## インストール

### ビルド済みバイナリを使う

[Releases](https://github.com/hiroyukim/green-pepper/releases)からOS/アーキテクチャに合った
アーカイブをダウンロードして展開すればよい(linux/darwin/windows × amd64/arm64)。

```sh
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')
url=$(curl -s https://api.github.com/repos/hiroyukim/green-pepper/releases/latest \
  | grep -o "https://[^\"]*green-pepper_[^\"]*_${os}_${arch}.tar.gz")
curl -sL "$url" | tar xz gp
sudo install -m 0755 gp /usr/local/bin/gp
gp --help
```

`/usr/local/bin`は多くの環境で標準的にPATHが通っている。別の場所に置きたい場合は、そのディレクトリを
`PATH`に追加する。

```sh
export PATH="$HOME/bin:$PATH"
```

Windowsの場合はReleasesページから`_windows_`のzipを直接ダウンロードする。

### ソースからビルドする

```sh
go build -o gp .
```

## 3分でわかる`gp serve`

### 1. 起動してブラウザを開く

```sh
gp serve examples/request.yaml --env examples/env.yaml --port 8080
```

`http://localhost:8080` を開くと、Method・URL・Headersなどを入力する画面が表示される。難しい設定は不要で、
上から順に埋めていくだけでよい。

![リクエストビルダーの画面。Method/URL/Authorization/Query Params/Headers/Body/Test Scriptを入力するフォームと、環境変数・CSV実行のカードが並ぶ](docs/images/request-builder.png)

### 2. 「単発送信」でまず1回試す

右下の「単発送信」ボタンを押すと、その場でリクエストが1回実行される。レスポンスはステータス・ヘッダー・
ボディまでその場で確認でき、JSONならきれいに整形&色付けして表示される(`Test Script`欄に検証コードを
書いておけば、ここに合否も一緒に出る)。

![レスポンス画面。ステータス200 OK、整形されたJSONボディ、2件のテスト結果(緑のチェックマーク)が表示されている](docs/images/response-viewer.png)

### 3. CSVをアップロードしてまとめて実行する

「CSVで一括実行」カードにCSVファイルをドラッグ&ドロップ(またはクリックして選択)すると、行数や列名を
事前にプレビューできる。「CSVで実行」を押すと、CSVの行数ぶんだけ同じリクエストを繰り返し実行する。

![CSVで一括実行カード。ドラッグ&ドロップ領域、反復回数・リクエスト間の遅延・失敗時に停止の入力欄](docs/images/csv-upload.png)

### 4. 進捗を見ながら待ち、結果を確認する

実行中は完了件数のプログレスバーが表示され、終わると自動的に結果画面に切り替わる。各行の「詳細を見る」を
押すと、その行のレスポンスヘッダー・ボディも個別に確認できる。

![実行中のプログレスバー。「実行中...」の文字と「2 / 3 件完了」の表示](docs/images/progress-bar.png)

![CSV実行結果のテーブル。行ごとにステータス・時間・テスト結果が並び、「詳細を見る」を展開するとレスポンスヘッダー・ボディが表示されている](docs/images/results-detail.png)

ここまでの操作は`curl`だけでも再現できる(自動化やCIから直接叩きたい場合はこちら)。

```sh
curl -X POST http://localhost:8080/execute \
  -F "method=GET" \
  -F "url={{base_url}}/users/{{id}}" \
  -F "headers=Accept: application/json" \
  -F "body=" \
  -F "env=base_url=https://jsonplaceholder.typicode.com" \
  -F "action=run" \
  -F "csv=@examples/users.csv"
```

スクリプトやAIエージェントから直接JSONで結果を受け取りたい場合は、`gp run --format json`か後述の
[`POST /api/run`](#json-api-apisend-apirun)を使うとよい。

## Web UI (`gp serve`) — 詳しいリファレンス

```sh
gp serve [request-file|collection-dir] [--env <env-file|env-dir>] [--port <port>] [--timeout <duration>]
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
- **CSVで一括実行** — CSVファイルをドラッグ&ドロップ(またはクリック)でアップロードすると、実行前に列名・行数・先頭数行をプレビューし、テンプレート変数がCSV・環境変数のどちらにもカバーされていない場合は警告する。「CSVで実行」を押すと`gp run`と同じ結果が表で表示される。コレクションディレクトリで起動している場合は、コレクション内の全リクエストをCSVの1行ごとに順番に実行し、結果テーブルに「Request」列でどのリクエストの結果かを表示する。「反復回数」「リクエスト間の遅延」「失敗時に停止」も指定でき、意味は`gp run`の`--iterations`/`--delay`/`--stop-on-error`と同じで、単一リクエスト実行・コレクション実行の両方に適用される。実行はバックグラウンドで進み、完了件数/全体件数のプログレスバーが表示され、終わると自動的にCSV実行履歴の結果画面に遷移する
- **環境変数編集・YAMLダウンロード** — 環境変数もブラウザ上で編集でき、編集中のリクエストはYAMLとしてダウンロードして`gp run`にそのまま使い回せる
- **複数環境の切り替え** — `--env`にディレクトリを渡すと、直下の`*.yaml`/`*.yml`ファイル(拡張子を除いたファイル名が環境名になる)がそれぞれ1つの環境として読み込まれ、「環境変数」カードにドロップダウンが表示される。切り替えると`#env`の内容がその環境の変数に置き換わり、「この環境を保存」で編集内容を元のファイルに書き戻せる。単一ファイル(または未指定)の場合、この操作は表示されず従来どおり

### 複数環境を切り替える(例)

```sh
mkdir -p envs
cat > envs/dev.yaml  <<'EOF'
base_url: https://dev.example.com
EOF
cat > envs/prod.yaml <<'EOF'
base_url: https://jsonplaceholder.typicode.com
EOF
gp serve examples/request.yaml --env envs --port 8080
```

「環境変数」カードに`dev`/`prod`のドロップダウンが表示され、選ぶと`#env`の内容がそのファイルの変数に
入れ替わる。切り替え自体はGETなので`curl`でも直接叩ける。

```sh
curl -s "http://localhost:8080/environment?name=prod" -o /dev/null -D - | head -1
```
```
HTTP/1.1 303 See Other
```

### 送信履歴(例)

単発送信するたびに履歴に記録され、トップ画面の「履歴」カードから過去のリクエストに一発で戻れる。

```sh
curl -s -X POST http://localhost:8080/execute \
  -F "method=GET" -F "url=https://jsonplaceholder.typicode.com/users/1" \
  -F "headers=" -F "body=" -F "env=" -F "action=send" -o /dev/null

curl -s -D - http://localhost:8080/history/1 -o /dev/null | head -1
```
```
HTTP/1.1 303 See Other
```

リダイレクト後の編集画面には、その時点のMethod/URL/Headers/Bodyがそのまま復元されている。

### コレクションで複数リクエストを連続実行(例)

`request-file`の代わりにディレクトリを渡すと、そのディレクトリ配下の`*.yaml`/`*.yml`ファイル群を
コレクションとして扱える（フラットな一覧のみで、フォルダの入れ子には対応しない)。

```sh
mkdir -p my-collection
cat > my-collection/get-user.yaml <<'EOF'
method: GET
url: "{{base_url}}/users/{{id}}"
headers:
  Accept: application/json
EOF
cat > my-collection/get-posts.yaml <<'EOF'
method: GET
url: "{{base_url}}/posts/{{id}}"
headers:
  Accept: application/json
EOF

gp serve my-collection --env examples/env.yaml
```

画面上部にコレクション内のリクエスト名一覧が表示され、選ぶとそのリクエストが編集画面に読み込まれる。
編集中の内容は「名前を付けて保存」でコレクションディレクトリにYAMLとして保存・上書きできる。
単一ファイルまたは無指定で起動した場合はこれまで通りで、コレクション機能は表示されない。

コレクション(複数リクエスト)とCSVを組み合わせると、CSVの1行につきコレクション内の全リクエストを
順番に実行する——コレクションをデータファイルで回す、という動き。CLIでも同じことができる。

```sh
gp run my-collection --data examples/users.csv --env examples/env.yaml
```
```
#  REQUEST    STATUS         TIME  SIZE  id    ERROR
1  get-posts  200 OK         50ms  292   1
1  get-user   200 OK         15ms  509   1
2  get-posts  200 OK         12ms  278   2
2  get-user   200 OK         12ms  509   2
3  get-posts  404 Not Found  13ms  2     9999
3  get-user   404 Not Found  12ms  2     9999

4/6 passed
```

`--iterations`はCSV全体をこの回数だけ繰り返す。2行のCSVで`--iterations 2`なら、行の順序は
`1, 2, 9999, 1, 2, 9999`のように2周する(単一リクエストでもコレクションでも同じ挙動)。
`gp serve`で実行する場合は、結果テーブルに「Request」列でどちらのリクエストの結果かが表示される。

### 進捗表示・CSV実行履歴・行ごとの詳細(例)

`gp serve`でのCSV/コレクション実行は非同期で行われる。フォーム送信は即座に進捗ページを返し、
ブラウザはバックグラウンドで`GET /run-progress/{id}`をポーリングして完了件数を更新する。

```sh
curl -s -X POST http://localhost:8080/execute \
  -F "method=GET" -F "url=x" -F "headers=" -F "body=" \
  -F "env=base_url=https://jsonplaceholder.typicode.com" \
  -F "action=run" -F "csv=@examples/users.csv" -o progress.html

grep -o 'data-run-id="[0-9]*"' progress.html
```
```
data-run-id="1"
```
```sh
curl -s http://localhost:8080/run-progress/1
```
```json
{"completed":2,"total":6,"done":false}
```
```json
{"completed":6,"total":6,"done":true,"historyId":1}
```

`done`が`true`になると`historyId`が入り、ブラウザは自動的に`/csv-history/{id}`(結果画面)へ遷移する。
このページは「CSV実行履歴」カードから後で見返すこともでき、各行は`<details>`で展開すると
そのリクエストのレスポンスヘッダー・ボディを個別に確認できる(JS不要、ブラウザ標準機能)。

```sh
curl -s http://localhost:8080/csv-history/1 | grep -o '[0-9]/[0-9] passed'
```
```
4/6 passed
```

### JSON API (`/api/send`, `/api/run`)

`gp serve`はブラウザ向けのHTML画面に加えて、スクリプトやAIエージェントから使うための
JSON専用エンドポイントも提供する。人間が使う場合はブラウザUIか`gp run --format json`を
使えばよく、これらはあくまで自動化・AIエージェント連携向け。

- **`POST /api/send`** — JSONボディでリクエストを1回実行し、結果をJSONで返す。UIの「単発送信」に相当する
- **`POST /api/run`** — `multipart/form-data`でリクエストとCSVファイルを渡して一括実行し、
  各行の結果をJSON配列で返す。UIの「CSVで実行」に相当する

いずれも**ステートレス**——リクエストボディ/フォームに渡した内容だけを実行し、
`POST /execute`と違って画面に表示中のリクエスト・環境変数（サーバー内部の状態）は一切変更しない。
そのため、ブラウザで編集中の内容に影響を与えずに、スクリプトから何度呼び出しても安全。

`POST /api/send`の例:

```sh
curl -X POST http://localhost:8080/api/send \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "url": "{{base_url}}/users/{{id}}",
    "headers": {"Accept": "application/json"},
    "env": {"base_url": "https://jsonplaceholder.typicode.com", "id": "1"}
  }'
```

```json
{"status":"200 OK","statusCode":200,"ok":true,"durationMs":42.5,"bytes":509,"headers":{"Content-Type":["application/json; charset=utf-8"]},"body":"...","error":""}
```

`POST /api/run`の例（`headers`/`env`はJSONオブジェクトを文字列にしたフォームフィールドとして渡す）:

```sh
curl -X POST http://localhost:8080/api/run \
  -F "method=GET" \
  -F "url={{base_url}}/users/{{id}}" \
  -F 'env={"base_url":"https://jsonplaceholder.typicode.com"}' \
  -F "csv=@examples/users.csv"
```

```json
[{"index":1,"status":"200 OK","statusCode":200,"ok":true,"durationMs":13.0,"bytes":509,"row":{"id":"1"},"error":""}, ...]
```

いずれのエンドポイントも、リクエストボディ/フォームが不正な場合はHTMLではなく
`{"error": "..."}`形式のJSONを400で返す。

## CLI (`gp run`)

CSVの各行で変数展開しながらリクエストテンプレートを繰り返し実行し、結果を表で表示する。CIでの疎通確認などに向く。

```sh
gp run <request-file> [--data <csv-file>] [--env <env-file|env-dir>] [--env-name <name>] [--timeout <duration>] [--format table|json] [--iterations <n>] [--delay <duration>] [--stop-on-error]
```

- `<request-file>`: リクエストテンプレート (YAML)。`method` / `url` / `headers` / `body` に `{{var}}` を書ける。ディレクトリ(コレクション)を渡すと、その直下の`*.yaml`/`*.yml`ファイル全てをファイル名の昇順で、CSVの1行ごとに順番に実行する(例: `gp run ./my-collection/ --data users.csv`)。結果の表・JSON出力には、どのリクエストの結果かを示す`REQUEST`列/`request`フィールドが加わる。
- `--data`: CSVファイル。1行目がヘッダー（変数名）、以降の各行が1リクエスト分の変数値。省略時は1回だけ実行する。
- `--env`: デフォルト変数を定義するYAML (`base_url` など)。CSVの値がある場合はそちらが優先される。ディレクトリを渡すと、直下の`*.yaml`/`*.yml`ファイルをそれぞれ1つの環境として扱う（Dev/Staging/Prodなど）。
- `--env-name`: `--env`がディレクトリのとき、使用する環境名を指定する。ファイルが1つしかない場合は省略可能（自動選択）。複数ある場合は必須で、未指定または存在しない名前を指定するとエラーになり、利用可能な環境名の一覧が表示される。単一ファイル指定時は無効。
- `--timeout`: リクエストごとのタイムアウト（デフォルト30秒）。
- `--format`: 出力形式。`table`（デフォルト、人間向けの表）か `json`（各リクエストの結果を構造化JSONの配列で出力。AIエージェントなどからの利用向け）。
- `--iterations`: 反復回数（デフォルト1）。`--data`指定時はCSV全体をこの回数だけ繰り返す（例: 2行のCSVで`--iterations 3`なら計6回、行0,1,0,1,0,1の順）。`--data`未指定時は同じリクエストをこの回数だけ繰り返す。
- `--delay`: リクエスト間の遅延（デフォルト0、`500ms`のような形式）。2回目以降の各リクエスト実行前に指定時間待機する。
- `--stop-on-error`: 指定すると、最初に失敗したリクエストの時点で実行を打ち切る（デフォルトは最後まで継続）。それまでの結果は表示され、終了コードにも反映される。

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

## テストスクリプト (`test_script`)

リクエストテンプレートに`test_script`を書くと、レスポンスを受け取るたびに[goja](https://github.com/dop251/goja)
(純Go実装のECMAScript処理系)でそのJavaScriptを実行し、`pm.test(...)`のような書き味でレスポンスを検証できる。
`method` / `url` / `headers` / `body`と違い、`test_script`は`{{var}}`テンプレート展開の対象外——変数は
`pm.variables.get(name)`経由で参照する。

```yaml
method: GET
url: "{{base_url}}/users/{{id}}"
test_script: |
  pm.test("status is 200", function () {
    if (pm.response.code !== 200) throw new Error("expected 200, got " + pm.response.code);
  });
  pm.test("body has a name field", function () {
    var body = pm.response.json();
    if (!body.name) throw new Error("missing name");
  });
  pm.test("row id matches", function () {
    var id = pm.variables.get("id");
    if (String(pm.response.json().id) !== String(id)) throw new Error("id mismatch");
  });
```

利用できるAPIは以下のみ(`pm.*`の完全なAPIセットを再現するものではない)。

- `pm.response.code` / `pm.response.status`: ステータスコード（数値）とステータス行（文字列）
- `pm.response.body`: レスポンスボディの文字列
- `pm.response.json()`: ボディを`JSON.parse`した値。不正なJSONの場合は例外を投げる
- `pm.variables.get(name)`: そのリクエスト実行時点の変数（CSVの行+環境変数のマージ済み）を参照する。無ければ空文字列
- `pm.test(name, fn)`: `fn`内で例外を投げれば失敗、投げなければ成功として、名前付きのテスト結果を記録する

`pm.test`は1つ失敗しても以降の`pm.test`の実行は止めない。スクリプト全体には5秒のタイムアウトがあり、無限ループなどで
時間切れになった場合や、構文エラー・`pm.test`の外で例外が起きた場合は`test_script`という名前の1件の失敗として記録される
（`gp run`/`gp serve`プロセス自体がクラッシュしたりハングしたりすることはない）。

テストが1つでも失敗すると、そのリクエストのステータスコードが2xxであっても失敗扱いになり、`gp run`の終了コード・
`--stop-on-error`の判定に反映される。`test_script`が無いリクエストの合否判定はステータスコードのみで、これまでと
完全に同じ。

結果はCLIの表・JSON出力に`TESTS`列/`tests`フィールドとして表示され、`gp serve`の単発送信・CSV実行の結果画面にも
反映される。

### デバッグ出力 (`console.log`)

`test_script`の中では`console.log(...)` / `console.warn(...)` / `console.error(...)`が使える。複数の引数はスペース
区切りで連結され、オブジェクトは可能であればJSON文字列化される。`warn`/`error`は行の先頭に`[warn]`/`[error]`が付く
以外は`log`と同じ扱いで、レベルを問わず1回の実行分としてまとめて記録される（記録できる行数には上限があり、
超えた分は`...(log limit reached)`という1行にまとめられる)。

```yaml
test_script: |
  console.log("checking id", pm.variables.get("id"));
  pm.test("status is 200", function () {
    if (pm.response.code !== 200) throw new Error("expected 200, got " + pm.response.code);
  });
```

記録された出力は`gp run --format json`（および`gp serve`のJSONエクスポート）の`logs`フィールドに配列として入る
（表形式`table`には出ない）。`gp serve`では単発送信のレスポンス画面にも「コンソール出力」として表示される。
CSV/コレクション実行の各行での表示は今のところ対象外。

## License

[Apache License 2.0](LICENSE)
