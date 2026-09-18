# CLAUDE.md

このファイルは、このリポジトリでの開発・保守を行う人(山中さん)とAIエージェント向けのメモ。
利用者向けのドキュメントは[README.md](README.md)を参照。

## フロントエンド (`gp serve`)

`gp serve`のWeb UIのJS/CSS/アイコンフォントは[frontend/](frontend/)以下の別プロジェクト
(esbuildでビルド)。`npm run build`の成果物を`internal/server/static/dist`に出力し、
`internal/server/server.go`が`go:embed`でバイナリに埋め込む。`frontend/src`を変更したら
`cd frontend && npm run build`をコミット前に実行すること(`internal/server/static/dist`は
`.gitignore`対象なのでコミットには含まれない)。CI ([ci.yml](.github/workflows/ci.yml)) と
リリース ([release.yml](.github/workflows/release.yml)) はどちらもNode.jsをセットアップして
`npm ci && npm run build`をGoのビルド・テストより前に実行する。

## リリース手順

`v*` 形式のタグをpushすると、GitHub Actions ([goreleaser](https://goreleaser.com/)) が
linux/darwin/windows × amd64/arm64 のバイナリをビルドし、GitHub Releaseに自動でアップロードする
([.github/workflows/release.yml](.github/workflows/release.yml)、設定は
[.goreleaser.yaml](.goreleaser.yaml))。

```sh
git tag v0.2.0
git push origin v0.2.0
```

アーカイブ名は`green-pepper_<version>_<os>_<arch>.tar.gz`(Windowsのみ`.zip`)。README.mdの
「ビルド済みバイナリを使う」の手順はこの命名規則に依存しているので、goreleaserの`archives`設定
(`name_template`やid)を変更した場合はREADME側のcurlコマンドも合わせて見直すこと。
