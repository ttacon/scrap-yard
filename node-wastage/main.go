package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
	humanize "github.com/dustin/go-humanize"
)

var (
	dir = flag.String("dir", "", "root to work from")
)

func main() {
	flag.Parse()

	validateFlagsOrExit()

	if err := work(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func validateFlagsOrExit() {
	if len(*dir) == 0 {
		fmt.Println("no directory given, exiting...")
		os.Exit(1)
	}
}

func work() error {
	root := *dir

	dirs, err := ioutil.ReadDir(root)
	if err != nil {
		return err
	}

	var projectsToCheck []os.FileInfo
	for _, element := range dirs {
		if element.IsDir() {
			projectsToCheck = append(projectsToCheck, element)
		}
	}

	fmt.Printf("found %d projects to check\n", len(projectsToCheck))

	start := time.Now()
	totalProcessed := 0

	bar := pb.StartNew(len(projectsToCheck))
	var data = make(map[string][]NodeUsageInfo)
	for _, proj := range projectsToCheck {
		projPath := filepath.Join(root, proj.Name())

		projFiles, err := ioutil.ReadDir(projPath)
		if err != nil {
			return err
		}

		hasPkgJSON := false
		hasNodeModules := false
		for _, file := range projFiles {
			if file.Name() == "package.json" {
				hasPkgJSON = true
			} else if file.Name() == "node_modules" {
				hasNodeModules = true
			}
		}
		if !hasPkgJSON || !hasNodeModules {
			bar.Increment()
			continue
		}

		processed, err := traverseInstalledPkgs(data, filepath.Join(
			projPath,
			"node_modules",
		))
		if err != nil {
			return err
		}

		totalProcessed += processed

		bar.Increment()
	}
	bar.Finish()
	fmt.Printf("processed %d entries in %s\n", totalProcessed, time.Now().Sub(start))

	fmt.Println("formatting results...")

	var results = make([]OverallNodeUsage, len(data))
	i := 0
	for _, usage := range data {
		results[i] = OverallNodeUsage{
			pkgName:    usage[0].pkgName,
			pkgVersion: usage[0].pkgVersion,
			info:       usage,
			size:       usage[0].dataSize,
		}
		i++
	}

	sort.Sort(ByName(results))

	f, err := os.Create("results.txt")
	if err != nil {
		return err
	}

	var globalUsage uint64
	for _, pkgUsage := range results {
		numInstances := len(pkgUsage.info)
		pkgSize := uint64(pkgUsage.size)
		pkgSizeHumanized := humanize.Bytes(pkgSize)
		totalSize := uint64(numInstances) * pkgSize
		totalSizeHumanized := humanize.Bytes(totalSize)

		f.WriteString(fmt.Sprintf(
			"%s@%s: %d (%s -> %s)\n",
			pkgUsage.pkgName,
			pkgUsage.pkgVersion,
			numInstances,
			pkgSizeHumanized,
			totalSizeHumanized,
		))

		globalUsage += totalSize
	}
	f.Sync()

	fmt.Printf("total space used: %s\n", humanize.Bytes(globalUsage))
	return f.Close()
}

type OverallNodeUsage struct {
	pkgName    string
	pkgVersion string
	info       []NodeUsageInfo
	size       int64
}

type ByName []OverallNodeUsage

func (a ByName) Len() int      { return len(a) }
func (a ByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool {
	return strings.Compare(a[i].pkgName, a[j].pkgName) < 1
}

func traverseInstalledPkgs(data map[string][]NodeUsageInfo, proj string) (int, error) {
	pkgs, err := ioutil.ReadDir(proj)
	if err != nil {
		return -1, err
	}

	for _, pkg := range pkgs {
		if !pkg.IsDir() {
			continue
		}

		raw, err := ioutil.ReadFile(filepath.Join(
			proj,
			pkg.Name(),
			"package.json",
		))
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return -1, err
		}

		var pkgInfo = make(map[string]interface{})
		if err := json.Unmarshal(raw, &pkgInfo); err != nil {
			return -1, err
		}

		name := pkgInfo["name"].(string)
		version := pkgInfo["version"].(string)
		id := fmt.Sprintf("%s:%s", name, version)

		dataSize, err := DirSize(filepath.Join(
			proj,
			pkg.Name(),
		))
		if err != nil {
			return -1, err
		}

		nodeInfo := NodeUsageInfo{
			pkgName:    name,
			pkgVersion: version,
			location:   proj,
			dataSize:   dataSize,
		}
		data[id] = append(data[id], nodeInfo)
	}
	return len(pkgs), nil
}

type NodeUsageInfo struct {
	pkgName    string
	pkgVersion string
	location   string
	dataSize   int64
}

func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
