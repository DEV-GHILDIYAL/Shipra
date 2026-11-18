package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/DEV-GHILDIYAL/Shipra/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to server JSON config")
	check := flag.Bool("check-config", false, "validate config and exit")
	flag.Parse()
	if !*check || *configPath == "" {
		fmt.Fprintln(os.Stderr, "usage: shipra-server -check-config -config PATH")
		os.Exit(2)
	}
	if _, err := config.Load(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, "invalid config:", err)
		os.Exit(1)
	}
	fmt.Println("configuration valid")
}
