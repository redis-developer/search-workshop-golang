// Command prep downloads the WANDS dataset and builds the deterministic
// workshop sample: a product corpus (corpus.jsonl), the judged queries
// that exercise it (queries.tsv), and graded relevance judgments in TREC
// format (qrels.txt), all under data/.
//
// Usage:
//
//	go run ./cmd/prep                  # build (or reuse) the default sample
//	go run ./cmd/prep -refresh         # rebuild outputs from the raw files
//	go run ./cmd/prep -full            # full WANDS: all products, all judged queries
//	go run ./cmd/prep -list-sources    # print the source URLs and exit
//
// Environment:
//
//	SAMPLE_PRODUCT_COUNT  products in the sample (default 600)
//	SAMPLE_QUERY_COUNT    queries in the sample (default 24)
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/redis-developer/search-workshop-golang/internal/catalog"
)

const (
	productsURL = "https://raw.githubusercontent.com/wayfair/WANDS/main/dataset/product.csv"
	queriesURL  = "https://raw.githubusercontent.com/wayfair/WANDS/main/dataset/query.csv"
	labelsURL   = "https://raw.githubusercontent.com/wayfair/WANDS/main/dataset/label.csv"

	dataDir = "data"
	rawDir  = "data/raw"

	defaultProductCount = 600
	defaultQueryCount   = 24
)

func main() {
	refresh := flag.Bool("refresh", false, "rebuild processed outputs even if they exist")
	full := flag.Bool("full", false, "use the full WANDS dataset instead of the workshop sample")
	listSources := flag.Bool("list-sources", false, "print the WANDS source URLs and exit")
	flag.Parse()

	if *listSources {
		fmt.Println("WANDS sources (tab-separated despite the .csv extension):")
		fmt.Println("  products:  " + productsURL)
		fmt.Println("  queries:   " + queriesURL)
		fmt.Println("  judgments: " + labelsURL)
		return
	}

	if err := run(*refresh, *full); err != nil {
		fmt.Fprintln(os.Stderr, "prep:", err)
		os.Exit(1)
	}
}

func run(refresh, full bool) error {
	corpusPath := filepath.Join(dataDir, "corpus.jsonl")
	queriesPath := filepath.Join(dataDir, "queries.tsv")
	qrelsPath := filepath.Join(dataDir, "qrels.txt")

	if !refresh && exists(corpusPath) && exists(queriesPath) && exists(qrelsPath) {
		fmt.Println("data/ already prepared: reusing it (pass -refresh to rebuild)")
		return summarize(corpusPath, queriesPath, qrelsPath)
	}

	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return err
	}

	productsFile, err := fetch(productsURL, filepath.Join(rawDir, "product.csv"))
	if err != nil {
		return err
	}
	queriesFile, err := fetch(queriesURL, filepath.Join(rawDir, "query.csv"))
	if err != nil {
		return err
	}
	labelsFile, err := fetch(labelsURL, filepath.Join(rawDir, "label.csv"))
	if err != nil {
		return err
	}

	products, err := parseProducts(productsFile)
	if err != nil {
		return err
	}
	queries, err := parseQueries(queriesFile)
	if err != nil {
		return err
	}
	judgments, err := parseJudgments(labelsFile)
	if err != nil {
		return err
	}
	fmt.Printf("parsed WANDS: %d products, %d queries, %d judgments\n",
		len(products), len(queries), len(judgments))

	productCount := envInt("SAMPLE_PRODUCT_COUNT", defaultProductCount)
	queryCount := envInt("SAMPLE_QUERY_COUNT", defaultQueryCount)
	if full {
		productCount = len(products)
		queryCount = 0 // all judged queries
		fmt.Println("full mode: keeping every product and every judged query")
	}

	sample := catalog.BuildSample(products, queries, judgments, productCount, queryCount)

	if err := catalog.WriteCorpus(corpusPath, sample.Products); err != nil {
		return err
	}
	if err := catalog.WriteQueries(queriesPath, sample.Queries); err != nil {
		return err
	}
	if err := catalog.WriteQrels(qrelsPath, sample.Judgments); err != nil {
		return err
	}
	return summarize(corpusPath, queriesPath, qrelsPath)
}

func summarize(corpusPath, queriesPath, qrelsPath string) error {
	products, err := catalog.ReadCorpus(corpusPath)
	if err != nil {
		return err
	}
	queries, err := catalog.ReadQueries(queriesPath)
	if err != nil {
		return err
	}
	judgments, err := catalog.ReadQrels(qrelsPath)
	if err != nil {
		return err
	}

	relevant := 0
	for _, j := range judgments {
		if j.Grade > 0 {
			relevant++
		}
	}
	fmt.Printf("workshop sample ready under %s/:\n", dataDir)
	fmt.Printf("  corpus.jsonl  %d products\n", len(products))
	fmt.Printf("  queries.tsv   %d judged queries\n", len(queries))
	fmt.Printf("  qrels.txt     %d judgments (%d relevant)\n", len(judgments), relevant)
	return nil
}

func parseProducts(path string) ([]catalog.Product, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	return catalog.ParseProducts(f)
}

func parseQueries(path string) ([]catalog.Query, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	return catalog.ParseQueries(f)
}

func parseJudgments(path string) ([]catalog.Judgment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	return catalog.ParseJudgments(f)
}

// fetch downloads url to path unless it is already present (raw files are
// cached so -refresh does not re-download ~200 MB of TSVs).
func fetch(url, path string) (string, error) {
	if exists(path) {
		fmt.Printf("using cached %s\n", path)
		return path, nil
	}
	fmt.Printf("downloading %s ...\n", url)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: unexpected status %s", url, resp.Status)
	}

	tmp := path + ".partial"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()      //nolint:errcheck // best effort on the error path
		os.Remove(tmp) //nolint:errcheck // best effort on the error path
		return "", fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func envInt(name string, fallback int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		fmt.Fprintf(os.Stderr, "prep: ignoring invalid %s=%q\n", name, os.Getenv(name))
	}
	return fallback
}
