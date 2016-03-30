package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
)

type NPMPackageFile struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Main         string            `json:"main"`
	Author       string            `json:"author"`
	License      string            `json:"license"`
	Homepage     string            `json:"homepage"`
	Dependencies map[string]string `json:"dependencies"`
}

type Packages struct {
	TotalRows int   `json:"total_rows"`
	Offset    int   `json:"offset"`
	Rows      []Row `json:"rows"`
}

type Row struct {
	Id    string `json:"id"`
	Key   string `json:"key"`
	Value struct {
		Rev string `json:"rev"`
	} `json:"value"`
}

func main() {
	outputLocation := os.Args[1]
	packageNameRoot := os.Args[2]
	version := os.Args[3]
	author := os.Args[4]

	if len(os.Args) < 4 {
		fmt.Println("Invalid arguments. npm-gen-all <output root> <package name base> <version #> <author name>")
		return
	}

	fmt.Println("Downloading NPM package list. This may take a while....")

	createNPMPackages(outputLocation, packageNameRoot, author, version, "A package that gets all packages.", "WTFPL")
}

func createNPMPackages(location, packageName, author, version, description, license string) {
	jobs := make(chan string, 100)
	// go publishNPM(jobs)

	resp, err := http.Get("https://skimdb.npmjs.com/registry/_all_docs/")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Got a bad status code")
		panic(resp.Status)
	}

	packagesJSON := Packages{}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(body, &packagesJSON)
	if err != nil {
		panic(err)
	}

	chunks := slicer(packagesJSON.Rows, 1000)
	fmt.Println(len(chunks))

	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(i int, packageName, location, description, version, author, license string, chunk *[]interface{}, jobs chan<- string) {
			defer wg.Done()
			name := packageName + "-" + strconv.Itoa(i)
			exportNPMPackage(chunk, location, name, description, version, author, license, jobs)
		}(i, packageName, location, description, version, author, license, &chunk, jobs)
	}

	wg.Wait()
}

func exportNPMPackage(rows *[]interface{}, location, packageName, description, version, author, license string, jobs chan<- string) {
	packages := make(map[string]string)

	fmt.Printf("Creating %v\n", packageName)

	for _, row := range *rows {
		r := row.(Row)
		packages[r.Id] = "*"
	}

	npmPackage := NPMPackageFile{}
	npmPackage.Name = packageName
	npmPackage.Dependencies = packages
	npmPackage.Description = description
	npmPackage.Version = version
	npmPackage.Author = author
	npmPackage.License = license
	npmPackage.Homepage = "http://github.com/ell/npm-gen-all"

	j, err := json.MarshalIndent(npmPackage, "", "  ")
	if err != nil {
		panic(err)
	}

	rootPath := filepath.Join(location, packageName)
	os.MkdirAll(rootPath, 0777)

	path := filepath.Join(rootPath, "package.json")
	helloPath := filepath.Join(rootPath, "index.js")
	readMePath := filepath.Join(rootPath, "README.md")

	helloWorld := []byte("console.log('Hello NPM :)')")
	readme := []byte("Yo, these are some cool packages imo. Lets make sure we have a nice backup it'd be a shame if someone unpublished them.")

	err = ioutil.WriteFile(helloPath, helloWorld, 0644)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(path, j, 0644)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(readMePath, readme, 0644)
	if err != nil {
		panic(err)
	}

	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		panic(err)
	}

	publishNPM(absPath)

	return

	/*cmd := exec.Command("npm", "publish", absPath)
	stdout, err := cmd.Output()

	if err != nil {
		panic(err)
	}

	print(string(stdout))
	*/
}

func publishNPM(path string) {
	cmd := exec.Command("npm", "publish", path)
	stdout, err := cmd.Output()

	fmt.Println(string(stdout))

	if err != nil {
		panic(err)
	}

	return
}

func slicer(a []Row, b int) [][]interface{} {
	val := reflect.ValueOf(a)

	origLen := val.Len()
	outerLen := origLen / b
	if origLen%b > 0 {
		outerLen++
	}

	c := make([][]interface{}, outerLen)

	for i := range c {
		newLen := b
		if origLen-(i*b+newLen) < 0 {
			newLen = origLen % b
		}
		c[i] = make([]interface{}, newLen)
		for j := range c[i] {
			c[i][j] = val.Index(i*b + j).Interface()
		}
	}

	return c
}
