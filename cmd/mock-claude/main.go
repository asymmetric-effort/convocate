package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(Version)
		return
	}

	// mock-claude: deterministic Claude Code stand-in for e2e testing.
	// Reads prompts from stdin (or --print mode) and returns canned responses.
	// Does not consume Anthropic API credits.

	if len(os.Args) > 1 && os.Args[1] == "--print" {
		// --print mode: read entire stdin and produce a deterministic response.
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		prompt := strings.Join(lines, "\n")
		fmt.Println(generateResponse(prompt))
		return
	}

	// Interactive mode: read line by line, respond to each.
	fmt.Fprintf(os.Stderr, "mock-claude %s (deterministic test mode)\n", Version)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fmt.Println(generateResponse(line))
	}
}

func generateResponse(prompt string) string {
	promptLower := strings.ToLower(prompt)

	// Canned responses based on prompt content.
	switch {
	case strings.Contains(promptLower, "background task:"):
		return "I'll implement the requested changes. Creating feature branch and working on the solution."

	case strings.Contains(promptLower, "fix") || strings.Contains(promptLower, "bug"):
		return "I've identified the bug and applied a fix. The issue was in the error handling path. Tests pass."

	case strings.Contains(promptLower, "feature") || strings.Contains(promptLower, "add"):
		return "I've implemented the requested feature. All tests pass and the changes are ready for review."

	case strings.Contains(promptLower, "clarif"):
		return "I need clarification on the acceptance criteria. What should happen when the input is empty?"

	case strings.Contains(promptLower, "test"):
		return "I've added tests for the new functionality. Coverage is above the target threshold."

	default:
		return "I've completed the requested work. Changes are committed and ready for review."
	}
}
