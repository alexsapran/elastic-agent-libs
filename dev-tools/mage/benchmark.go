package mage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/elastic/elastic-agent-libs/dev-tools/mage/gotool"
	"github.com/magefile/mage/mg"
)

const (
	goBenchstat = "golang.org/x/perf/cmd/benchstat@v0.0.0-20230227161431-f7320a6d63e8"
)

var (
	benchmarkCount = 8
)

// Benchmark namespace for mage to group all the related targets under this namespace
type Benchmark mg.Namespace

// Deps installs required plugins for reading benchmarks results
func (Benchmark) Deps() error {
	err := gotool.Install(gotool.Install.Package(goBenchstat))
	if err != nil {
		return err
	}
	return nil
}

// Run execute the go benchmark tests for this repository, define OUTPUT to write results into a file
func (Benchmark) Run(ctx context.Context) error {
	mg.Deps(Benchmark.Deps)
	fmt.Println(">> go Test:", "Benchmark")
	outputFile := os.Getenv("OUTPUT")
	benchmarkCountOverride := os.Getenv("BENCH_COUNT")
	if benchmarkCountOverride != "" {
		var overrideErr error
		benchmarkCount, overrideErr = strconv.Atoi(benchmarkCountOverride)
		if overrideErr != nil {
			return fmt.Errorf("failed to parse BENCH_COUNT, verify that you set the right value: , %w", overrideErr)
		}
	}
	projectPackages, er := gotool.ListProjectPackages()
	if er != nil {
		return fmt.Errorf("failed to list package dependencies: %w", er)
	}
	cmdArg := fmt.Sprintf("test -count=%d -bench=Bench -run=Bench", benchmarkCount)
	cmdArgs := strings.Split(cmdArg, " ")
	for _, pkg := range projectPackages {
		cmdArgs = append(cmdArgs, filepath.Join(pkg, "/..."))
	}
	_, err := runCommand(ctx, nil, "go", outputFile, cmdArgs...)

	var goTestErr *exec.ExitError
	switch {
	case goTestErr == nil:
		return nil
	case errors.As(err, &goTestErr):
		return fmt.Errorf("failed to execute go test -bench command: %w", err)
	default:
		return fmt.Errorf("failed to execute go test -bench command %w", err)
	}
}

// Diff compare 2 benchmark outputs, Required BASE variable for parsing results, define NEXT to compare base with next and optional OUTPUT to write to file
func (Benchmark) Diff(ctx context.Context) error {
	mg.Deps(Benchmark.Deps)
	fmt.Println(">> running: benchstat")
	outputFile := os.Getenv("OUTPUT")
	baseFile := os.Getenv("BASE")
	nextFile := os.Getenv("NEXT")
	var args []string
	if baseFile == "" {
		log.Printf("Missing required parameter BASE parameter to parse the results. Please set this to a filepath of the benchmark results")
		return fmt.Errorf("missing required parameter BASE parameter to parse the results. Please set this to a filepath of the benchmark results")
	} else {
		args = append(args, baseFile)
	}
	if nextFile == "" {
		log.Printf("Missing NEXT parameter, we are not going to compare results")
	} else {
		args = append(args, nextFile)
	}

	_, err := runCommand(ctx, nil, "benchstat", outputFile, args...)

	var goTestErr *exec.ExitError
	switch {
	case goTestErr == nil:
		return nil
	case errors.As(err, &goTestErr):
		return fmt.Errorf("failed to execute benchstat command: %w", err)
	default:
		return fmt.Errorf("failed to execute benchstat command!! %w", err)
	}

}

// runCommand is executing a command that is represented by cmd.
// when defining an outputFile it will write the stdErr, stdOut of that command to the output file
// otherwise it will capture it to stdErr, stdOut of the console used.
func runCommand(ctx context.Context, env map[string]string, cmd string, outputFile string, args ...string) (*exec.Cmd, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Env = os.Environ()
	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}

	if outputFile != "" {
		fileOutput, err := os.Create(createDir(outputFile))
		if err != nil {
			return nil, fmt.Errorf("failed to create %s output file: %w", cmd, err)
		}
		defer func(fileOutput *os.File) {
			err := fileOutput.Close()
			if err != nil {
				log.Fatalf("Failed to close file %s", err)
			}
		}(fileOutput)
		c.Stdout = io.MultiWriter(os.Stdout, fileOutput)
		c.Stderr = io.MultiWriter(os.Stderr, fileOutput)

	} else {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}
	c.Stdin = os.Stdin

	log.Println("exec:", cmd, strings.Join(args, " "))
	fmt.Println("exec:", cmd, strings.Join(args, " "))

	exitCode := c.Run()
	return c, exitCode
}

// createDir creates the parent directory for the given file.
func createDir(file string) string {
	// Create the output directory.
	if dir := filepath.Dir(file); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create parent dir for %s", file)
		}
	}
	return file
}
