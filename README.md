# Podcast用音声ファイルダウンロードツール

## 概要

このツールはPodcastのRSSフィードファイルを読み取って、そのファイル内の音声ファイルをダウンロードします。
音声ファイルは各エピソードのメタデータを元にしたファイル名にします。

## 仕様

### 全体の仕様

* このツールはGo言語で記述されるコマンドラインツールです
* コマンドラインツールのフラッグは次のとおりです。必須とカッコ書きしてあるものは必須のフラグです。
  * -c, --config: 設定ファイルのパス。このオプションが指定されている場合はすべてに優先されます。
    * 設定ファイルの仕様は「設定ファイルの仕様」のセクションに記述
  * -u, --url: PodcastのRSSフィードのURLを指定します（必須）
  * -d, --dest: ダウンロードした音声ファイルを保存する先のディレクトリです。デフォルトはカレントディレクトリです。ディレクトリが存在しない場合は作成します。
  * -s, --since: 指定された日付（RFC3999形式）以降のファイルしかダウンロードしないようにする。デフォルトはRSSファイル内のファイルすべて。
  * -f, --format: ダウンロードするファイル名のフォーマット。仕様は「ファイル名の仕様」のセクションに記述します。
  * --validate: ダウンロードしたファイルがRSSフィードで指定された秒数あるかを確認する（ffmpegが使える場合のみ有効）

* 渡されたPodcastのRSSフィード（XML）をパースしメタデータを取得します
  * PodcastのRSSフィードは [PSP-1](https://github.com/Podcast-Standards-Project/PSP-1-Podcast-RSS-Specification) に従ったもののみ扱うとします
  * チャンネル（ `/rss/channel` 要素）のメタデータ
    * `/rss/channel/title` 要素: チャンネル名
  * 各エピソード（ `/rss/channel/item` 要素）内のメタデータ
    * `/rss/channel/item/enclosure` 要素: `url` 属性がダウンロードする音声ファイルのURLです
    * `/rss/channel/item/title` 要素: エピソードのタイトル
    * `/rss/channel/item/pubDate` 要素: エピソードの日付（RFC1123形式）
* ダウンロードしたファイルは「ファイル名の仕様」のセクションの仕様にしたがってファイル名をつけます。
  * デフォルトは `{channel}-{date}-{episode}.mp3` です。
* コマンドを実行したら `/rss/channel/item` の要素分ファイルのダウンロードを実行します。
  * 標準出力に次の内容を表示します
    * ダウンロード開始時にどのエピソードのダウンロードを開始するかを表示します
    * ダウンロード中に音声ファイルのURLを表示します
    * ダウンロード完了時に音声ファイルの保存先（ファイル名）を表示します

### 設定ファイルの仕様

設定ファイルはYAMLで指定します。YAMLのスキーマは次のとおりです。

```yaml
apiVersion: v1 # 固定
tasks: # 配列
  - url: string # PodcastのRSS配信先のURL
    destination: string # （オプション）ダウンロードしたファイルを格納するディレクトリパス。デフォルトはカレントディレクトリ。
    since: string # （オプション）ダウンロード対象とするエピソードはこの日付以降とする。RFC3999形式で記述。デフォルトは 1960-01-01T00:00:00+09:00。
    format: string # （オプション）ダウンロードしたファイルのファイル名のテンプレート。デフォルトは {title}-{date}-{episode}.mp3
```

次がサンプルのYAMLファイルです。

```yaml
apiVersion: v1
tasks:
  - url: https://feeds.megaphone.fm/unagerorin
    destination: /home/ymotongpoo/downloads/mayurika
    since: 2025-05-24T00:00:00+09:00
    format: "{title}-{date}-{episode}.mp3"
  - url: https://feeds.megaphone.fm/FNCOMMUNICATIONSINC3656403561
    destination: /home/ymotongpoo/downloads/haruhiko
    format: "{title}-{date}-{episode}.mp3"
```

直接オプションを指定する場合と異なり、設定ファイルの場合には1つのRSSを1つのタスクとして設定し、複数のPodcastに対してファイルのダウンロード作業を指定できます。

### ファイル名の仕様

ファイル名のフォーマットの指定には、PodcastのRSSの情報に基づいて、いくつかの変数が使えます。

* `{channel}`: `/rss/channel/title` のテキストを入れます。空白はアンダースコア（ `_` ）に変換します
* `{date}`: エピソードの日付をyyyymmdd形式にしたもの
  * 例: `/rss/channel/item/pubDate` が `Sat, 24 May 2025 13:57:00 -0000` のときは `20250524` になる
* `{episode}`: `/rss/channel/item/title` のテキストを入れます。空白はアンダースコア（ `_` ）に変換します
