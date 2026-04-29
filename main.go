package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lixiang810/wx_channels_download/pkg/downloader"
)

const version = "0.1.0"

func main() {
	// Command-line flags
	var (
		port    = flag.Int("port", 8080, "Port for the local proxy server to listen on")
		output  = flag.String("output", ".", "Directory to save downloaded videos")
		verbose = flag.Bool("verbose", false, "Enable verbose logging")
		showVer = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("wx_channels_download v%s\n", version)
		os.Exit(0)
	}

	// Validate output directory
	if err := ensureDir(*output); err != nil {
		log.Fatalf("Failed to create output directory %q: %v", *output, err)
	}

	if *verbose {
		log.Printf("wx_channels_download v%s starting...", version)
		log.Printf("Proxy port : %d", *port)
		log.Printf("Output dir : %s", *output)
	}

	// Initialize and start the downloader proxy
	d, err := downloader.New(downloader.Config{
		Port:    *port,
		Output:  *output,
		Verbose: *verbose,
	})
	if err != nil {
		log.Fatalf("Failed to initialize downloader: %v", err)
	}

	fmt.Printf("\nwx_channels_download v%s\n", version)
	fmt.Println("==========================================")
	fmt.Printf("Proxy server listening on http://127.0.0.1:%d\n", *port)
	fmt.Println("Configure your WeChat to use this proxy, then browse Channels videos.")
	fmt.Printf("Videos will be saved to: %s\n", *output)
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println("==========================================")

	if err := d.Run(); err != nil {
		log.Fatalf("Downloader error: %v", err)
	}
}

// ensureDir creates the directory at path if it does not already exist.
func ensureDir(path string) error {
	if path == "." || path == "" {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}
