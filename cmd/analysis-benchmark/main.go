package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/kyangconn/music-online-go/internal/analysisbench"
)

type resultPaths []string

func (paths *resultPaths) String() string { return fmt.Sprint([]string(*paths)) }

func (paths *resultPaths) Set(value string) error {
	if value == "" {
		return errors.New("result path cannot be empty")
	}
	*paths = append(*paths, value)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "analysis benchmark:", err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("analysis-benchmark", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "path to a benchmark gold-set manifest")
	format := flags.String("format", "markdown", "report format: markdown or json")
	var candidates resultPaths
	flags.Var(&candidates, "result", "candidate result JSON path; repeat for comparisons")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *manifestPath == "" || len(candidates) == 0 {
		return errors.New("-manifest and at least one -result are required")
	}
	manifestFile, err := os.Open(*manifestPath) // #nosec G304 -- explicit local CLI input.
	if err != nil {
		return fmt.Errorf("open manifest: %w", err)
	}
	manifest, digest, decodeErr := analysisbench.DecodeManifest(manifestFile)
	closeErr := manifestFile.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if closeErr != nil {
		return fmt.Errorf("close manifest: %w", closeErr)
	}
	runs := make([]*analysisbench.CandidateRun, 0, len(candidates))
	for _, path := range candidates {
		file, err := os.Open(path) // #nosec G304 -- explicit local CLI input.
		if err != nil {
			return fmt.Errorf("open result %q: %w", path, err)
		}
		run, decodeErr := analysisbench.DecodeCandidateRun(file)
		closeErr := file.Close()
		if decodeErr != nil {
			return fmt.Errorf("result %q: %w", path, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close result %q: %w", path, closeErr)
		}
		runs = append(runs, run)
	}
	report, err := analysisbench.Evaluate(manifest, digest, runs)
	if err != nil {
		return err
	}
	switch *format {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case "markdown":
		rendered, err := analysisbench.Markdown(report)
		if err != nil {
			return err
		}
		_, err = io.WriteString(stdout, rendered)
		return err
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}
}
