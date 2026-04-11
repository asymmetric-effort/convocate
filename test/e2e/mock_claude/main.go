// Package main provides a mock claude CLI binary for end-to-end testing.
// It mimics basic claude CLI behavior: prints a greeting, reads input, echoes it, and exits.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	// Handle version flag
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println("claude-mock 1.0.0 (test)")
			os.Exit(0)
		}
		if arg == "--help" || arg == "-h" {
			fmt.Println("mock claude CLI for testing")
			os.Exit(0)
		}
	}

	fmt.Println("Claude CLI (mock) - Ready")
	fmt.Printf("User: %s\n", os.Getenv("USER"))
	fmt.Printf("Home: %s\n", os.Getenv("HOME"))
	fmt.Printf("UID:  %d\n", os.Getuid())
	fmt.Println("")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "exit" || line == "quit" {
			fmt.Println("Goodbye!")
			break
		}
		if line == "" {
			fmt.Print("> ")
			continue
		}
		fmt.Printf("echo: %s\n", line)
		fmt.Print("> ")
	}
}
