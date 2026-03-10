package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const defaultServerURL = "http://localhost:8080"

type SearchResult struct {
	Title       string `json:"title"`
	Size        int64  `json:"size"`
	Seeders     int    `json:"seeders"`
	Peers       int    `json:"peers"`
	Grabs       int    `json:"grabs"`
	Category    string `json:"category"`
	PublishDate string `json:"publish_date"`
	MagnetLink  string `json:"magnet_link"`
	Indexer     string `json:"indexer"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Count   int            `json:"count"`
}

type FileMeta struct {
	Hash       string    `json:"hash"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	LastAccess time.Time `json:"last_access"`
}

type FilesResponse struct {
	Files []*FileMeta `json:"files"`
	Count int         `json:"count"`
}

type AddMagnetResponse struct {
	Status string `json:"status"`
	Hash   string `json:"hash"`
}

func main() {
	serverURL := flag.String("server", defaultServerURL, "URL сервера magnet-player")
	interactive := flag.Bool("i", false, "Интерактивный режим (REPL)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <command> [arguments]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  search <query>       Поиск торрентов через Jackett\n")
		fmt.Fprintf(os.Stderr, "  add <magnet>         Добавить magnet-ссылку\n")
		fmt.Fprintf(os.Stderr, "  list                 Список файлов в кеше\n")
		fmt.Fprintf(os.Stderr, "  remove <hash>        Удалить файл из кеша\n")
		fmt.Fprintf(os.Stderr, "  status               Статус сервера\n")
		fmt.Fprintf(os.Stderr, "  help                 Показать справку\n")
		fmt.Fprintf(os.Stderr, "  quit/exit            Выход из интерактивного режима\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *interactive {
		runInteractive(*serverURL)
		return
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	var err error
	switch cmd {
	case "search":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "error: query required")
			fmt.Fprintln(os.Stderr, "usage: search <query>")
			os.Exit(1)
		}
		err = cmdSearch(*serverURL, strings.Join(args, " "))
	case "add":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "error: magnet link required")
			fmt.Fprintln(os.Stderr, "usage: add <magnet-link>")
			os.Exit(1)
		}
		err = cmdAdd(*serverURL, args[0])
	case "list":
		err = cmdList(*serverURL)
	case "remove":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "error: hash required")
			fmt.Fprintln(os.Stderr, "usage: remove <hash>")
			os.Exit(1)
		}
		err = cmdRemove(*serverURL, args[0])
	case "status":
		err = cmdStatus(*serverURL)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		flag.Usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runInteractive(serverURL string) {
	fmt.Println("magnet-player CLI - интерактивный режим")
	fmt.Println("Введите 'help' для списка команд, 'quit' для выхода")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "quit" || line == "exit" {
			fmt.Println("Выход...")
			break
		}

		if line == "help" {
			printInteractiveHelp()
			continue
		}

		args := strings.Fields(line)
		cmd := args[0]
		cmdArgs := args[1:]

		var err error
		switch cmd {
		case "search":
			if len(cmdArgs) < 1 {
				fmt.Println("error: query required")
				continue
			}
			err = cmdSearch(serverURL, strings.Join(cmdArgs, " "))
		case "add":
			if len(cmdArgs) < 1 {
				fmt.Println("error: magnet link required")
				continue
			}
			err = cmdAdd(serverURL, cmdArgs[0])
		case "list":
			err = cmdList(serverURL)
		case "remove":
			if len(cmdArgs) < 1 {
				fmt.Println("error: hash required")
				continue
			}
			err = cmdRemove(serverURL, cmdArgs[0])
		case "status":
			err = cmdStatus(serverURL)
		default:
			fmt.Printf("unknown command: %s\n", cmd)
			continue
		}

		if err != nil {
			fmt.Printf("error: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading input: %v\n", err)
	}
}

func printInteractiveHelp() {
	fmt.Println("\nДоступные команды:")
	fmt.Println("  search <query>       Поиск торрентов через Jackett")
	fmt.Println("  add <magnet>         Добавить magnet-ссылку")
	fmt.Println("  list                 Список файлов в кеше")
	fmt.Println("  remove <hash>        Удалить файл из кеша")
	fmt.Println("  status               Статус сервера")
	fmt.Println("  help                 Показать эту справку")
	fmt.Println("  quit/exit            Выход")
	fmt.Println()
}

func cmdSearch(serverURL, query string) error {
	urlStr := fmt.Sprintf("%s/api/search?q=%s", serverURL, url.QueryEscape(query))
	resp, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Пытаемся распарсить JSON ошибку
		var errResp map[string]string
		if json.Unmarshal(body, &errResp) == nil && errResp["error"] != "" {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp["error"])
		}
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result SearchResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("No results found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "INDEXER\tTITLE\tSIZE\tSEEDERS\tPEERS")
	fmt.Fprintln(w, "-------\t-----\t----\t-------\t-----")

	for _, r := range result.Results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\n",
			r.Indexer,
			truncate(r.Title, 50),
			formatSize(r.Size),
			r.Seeders,
			r.Peers,
		)
	}
	w.Flush()

	fmt.Printf("\nFound: %d results\n", result.Count)
	return nil
}

func cmdAdd(serverURL, magnet string) error {
	reqBody := map[string]string{"magnet": magnet}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/add-magnet", serverURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	var result AddMagnetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("Added successfully\n")
	fmt.Printf("Hash: %s\n", result.Hash)
	fmt.Printf("Status: %s\n", result.Status)
	return nil
}

func cmdList(serverURL string) error {
	url := fmt.Sprintf("%s/api/files", serverURL)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	var result FilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("No files in cache")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HASH\tSIZE\tLAST ACCESS\tPATH")
	fmt.Fprintln(w, "----\t----\t-----------\t----")

	for _, f := range result.Files {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			truncate(f.Hash, 12),
			formatSize(f.Size),
			f.LastAccess.Format("2006-01-02 15:04"),
			truncate(f.Path, 40),
		)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d files, %s\n", result.Count, formatSize(totalSize(result.Files)))
	return nil
}

func cmdRemove(serverURL, hash string) error {
	url := fmt.Sprintf("%s/api/files/%s", serverURL, hash)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	fmt.Printf("File %s removed successfully\n", hash)
	return nil
}

func cmdStatus(serverURL string) error {
	url := fmt.Sprintf("%s/health", serverURL)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("server is not available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	fmt.Println("Server status: OK")
	fmt.Printf("Response: %s\n", string(body))
	return nil
}

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * 1024
		GB = 1024 * 1024 * 1024
		TB = 1024 * 1024 * 1024 * 1024
	)

	switch {
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/TB)
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func totalSize(files []*FileMeta) int64 {
	var total int64
	for _, f := range files {
		total += f.Size
	}
	return total
}

func parseSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	
	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSpace(numStr)
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, err
			}
			return int64(num * float64(mult)), nil
		}
	}

	num, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}
