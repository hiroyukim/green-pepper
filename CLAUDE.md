# CLAUDE.md

このファイルは、このリポジトリでの開発・保守を行う人(山中さん)とAIエージェント向けのメモ。
利用者向けのドキュメントは[README.md](README.md)を参照。

## リリース手順

`v*` 形式のタグをpushすると、GitHub Actions ([goreleaser](https://goreleaser.com/)) が
linux/darwin/windows × amd64/arm64 のバイナリをビルドし、GitHub Releaseに自動でアップロードする
([.github/workflows/release.yml](.github/workflows/release.yml)、設定は
[.goreleaser.yml](.goreleaser.yml))。

```sh
git tag v0.2.0
git push origin v0.2.0
```

アーカイブ名は`green-pepper_<version>_<os>_<arch>.tar.gz`(Windowsのみ`.zip`)。README.mdの
「ビルド済みバイナリを使う」の手順はこの命名規則に依存しているので、goreleaserの`archives`設定
(`name_template`やid)を変更した場合はREADME側のcurlコマンドも合わせて見直すこと。
