package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: coverage-check profile")
		os.Exit(2)
	}
	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	totals := map[string][2]int{}
	blocks := map[string][2]int{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		statements, err := strconv.Atoi(fields[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		prior := blocks[fields[0]]
		prior[0] = statements
		if count > 0 {
			prior[1] = statements
		}
		blocks[fields[0]] = prior
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for block, counts := range blocks {
		file, _, _ := strings.Cut(block, ":")
		pkg := path.Dir(file)
		sum := totals[pkg]
		sum[0] += counts[0]
		sum[1] += counts[1]
		totals[pkg] = sum
	}
	names := make([]string, 0, len(totals))
	for name := range totals {
		names = append(names, name)
	}
	sort.Strings(names)
	failed := len(names) == 0
	for _, name := range names {
		counts := totals[name]
		if counts[0] == 0 {
			continue
		}
		percent := 100 * float64(counts[1]) / float64(counts[0])
		fmt.Printf("%s: %.1f%%\n", name, percent)
		if percent < 85 {
			failed = true
		}
	}
	if failed {
		fmt.Fprintln(os.Stderr, "pure rules require at least 85% statement coverage per package")
		os.Exit(1)
	}
}
