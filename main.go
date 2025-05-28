package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RSS はPodcastのRSSフィードの構造を表します
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

// Channel はRSSフィード内のチャンネル情報を表します
type Channel struct {
	Title string `xml:"title"`
	Items []Item `xml:"item"`
}

// Item はRSSフィード内の各エピソード情報を表します
type Item struct {
	Title     string    `xml:"title"`
	PubDate   string    `xml:"pubDate"`
	Enclosure Enclosure `xml:"enclosure"`
}

// Enclosure はエピソード内の音声ファイル情報を表します
type Enclosure struct {
	URL string `xml:"url,attr"`
}

// Config は設定ファイルの構造を表します
type Config struct {
	APIVersion string `yaml:"apiVersion"`
	Tasks      []Task `yaml:"tasks"`
}

// Task は設定ファイル内の各タスクを表します
type Task struct {
	URL         string `yaml:"url"`
	Destination string `yaml:"destination"`
	Since       string `yaml:"since"`
	Format      string `yaml:"format"`
}

func main() {
	// コマンドラインフラグの定義
	configFile := flag.String("c", "", "設定ファイルのパス")
	flag.StringVar(configFile, "config", "", "設定ファイルのパス")
	
	rssURL := flag.String("u", "", "PodcastのRSSフィードのURL")
	flag.StringVar(rssURL, "url", "", "PodcastのRSSフィードのURL")
	
	saveDir := flag.String("d", ".", "ダウンロードした音声ファイルを保存する先のディレクトリ")
	flag.StringVar(saveDir, "dest", ".", "ダウンロードした音声ファイルを保存する先のディレクトリ")
	
	sinceDate := flag.String("s", "", "指定された日付（RFC3999形式）以降のファイルしかダウンロードしない")
	flag.StringVar(sinceDate, "since", "", "指定された日付（RFC3999形式）以降のファイルしかダウンロードしない")
	
	fileFormat := flag.String("f", "{channel}-{date}-{episode}.mp3", "ダウンロードするファイル名のフォーマット")
	flag.StringVar(fileFormat, "format", "{channel}-{date}-{episode}.mp3", "ダウンロードするファイル名のフォーマット")
	
	validate := flag.Bool("validate", false, "ダウンロードしたファイルがRSSフィードで指定された秒数あるかを確認する")
	
	flag.Parse()

	// 設定ファイルが指定されている場合は、そちらを優先
	if *configFile != "" {
		config, err := loadConfig(*configFile)
		if err != nil {
			fmt.Printf("設定ファイルの読み込みに失敗しました: %v\n", err)
			os.Exit(1)
		}
		
		// 設定ファイルに基づいて各タスクを実行
		for i, task := range config.Tasks {
			fmt.Printf("タスク %d/%d を実行中...\n", i+1, len(config.Tasks))
			
			// タスクのデフォルト値を設定
			if task.Destination == "" {
				task.Destination = "."
			}
			if task.Format == "" {
				task.Format = "{channel}-{date}-{episode}.mp3"
			}
			
			// タスクを実行
			if err := processTask(task, *validate); err != nil {
				fmt.Printf("タスク %d の実行に失敗しました: %v\n", i+1, err)
			}
		}
		
		return
	}

	// URLが指定されていない場合はエラー
	if *rssURL == "" {
		fmt.Println("エラー: 設定ファイル(-c/--config)またはRSS URL(-u/--url)のいずれかを指定してください")
		flag.Usage()
		os.Exit(1)
	}

	// コマンドラインオプションからタスクを作成
	task := Task{
		URL:         *rssURL,
		Destination: *saveDir,
		Since:       *sinceDate,
		Format:      *fileFormat,
	}
	
	// タスクを実行
	if err := processTask(task, *validate); err != nil {
		fmt.Printf("タスクの実行に失敗しました: %v\n", err)
		os.Exit(1)
	}
}

// loadConfig は設定ファイルを読み込みます
func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	// APIVersionの確認
	if config.APIVersion != "v1" {
		return nil, fmt.Errorf("サポートされていないAPIバージョン: %s", config.APIVersion)
	}
	
	return &config, nil
}

// processTask は1つのタスクを処理します
func processTask(task Task, validate bool) error {
	// 保存先ディレクトリが存在しない場合は作成
	if _, err := os.Stat(task.Destination); os.IsNotExist(err) {
		if err := os.MkdirAll(task.Destination, 0755); err != nil {
			return fmt.Errorf("ディレクトリの作成に失敗しました: %v", err)
		}
	}

	// sinceDate が指定されている場合は解析
	var since time.Time
	if task.Since != "" {
		var err error
		since, err = time.Parse(time.RFC3339, task.Since)
		if err != nil {
			return fmt.Errorf("日付の解析に失敗しました: %v", err)
		}
	}

	// RSSフィードの取得
	rss, err := fetchRSS(task.URL)
	if err != nil {
		return fmt.Errorf("RSSフィードの取得に失敗しました: %v", err)
	}

	// 各エピソードの音声ファイルをダウンロード
	for _, item := range rss.Channel.Items {
		fmt.Printf("エピソード「%s」のダウンロードを開始します\n", item.Title)

		// 日付の解析
		pubDate, err := time.Parse(time.RFC1123, item.PubDate)
		if err != nil {
			// RFC1123Z形式も試す
			pubDate, err = time.Parse(time.RFC1123Z, item.PubDate)
			if err != nil {
				fmt.Printf("日付の解析に失敗しました: %v\n", err)
				continue
			}
		}

		// sinceDate が指定されていて、エピソードの日付がそれより前の場合はスキップ
		if !since.IsZero() && pubDate.Before(since) {
			fmt.Printf("エピソード「%s」は指定された日付より前のためスキップします\n", item.Title)
			continue
		}

		dateStr := pubDate.Format("20060102")

		// ファイル名の作成
		// 半角空白と全角空白を_に置換
		channelTitle := strings.ReplaceAll(strings.ReplaceAll(rss.Channel.Title, " ", "_"), "　", "_")
		episodeTitle := strings.ReplaceAll(strings.ReplaceAll(item.Title, " ", "_"), "　", "_")
		
		// ファイル名のフォーマットを適用
		fileName := task.Format
		fileName = strings.ReplaceAll(fileName, "{channel}", channelTitle)
		fileName = strings.ReplaceAll(fileName, "{date}", dateStr)
		fileName = strings.ReplaceAll(fileName, "{episode}", episodeTitle)
		
		filePath := filepath.Join(task.Destination, fileName)

		// 音声ファイルのダウンロード
		fmt.Printf("ダウンロード中: %s\n", item.Enclosure.URL)
		if err := downloadFile(item.Enclosure.URL, filePath); err != nil {
			fmt.Printf("ダウンロードに失敗しました: %v\n", err)
			continue
		}

		fmt.Printf("ダウンロード完了: %s\n", filePath)
		
		// ファイルの検証（--validate オプションが指定されている場合）
		if validate {
			fmt.Println("ファイル検証機能はまだ実装されていません")
		}
	}
	
	return nil
}

// fetchRSS はRSSフィードを取得してパースします
func fetchRSS(url string) (*RSS, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTPステータスコード: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rss RSS
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}

	return &rss, nil
}

// downloadFile はURLからファイルをダウンロードして保存します
func downloadFile(url, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTPステータスコード: %d", resp.StatusCode)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}
