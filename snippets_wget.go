package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

func main() {


	fmt.Println("Raw Args data - count: ", len(os.Args), "; Values: ", os.Args, "Type: ", reflect.TypeOf(os.Args))
	// os.Exit(0)
	var (
		isBackground  bool
		localFilename string
		localPath     string
		rateLimit     string
		inputFile     string
	)
	flag.BoolVar(&isBackground, "B", false, "Download the file in the background without printout to screen.")
	flag.StringVar(&localFilename, "O", "", "Save downloaded data under the specified filename.")
	flag.StringVar(&localPath, "P", "./", "Path where the downloaded data will be saved.")
	flag.StringVar(&rateLimit, "rate-limit", "", "Download speed limit.")
	flag.StringVar(&inputFile, "i", "", "Filename to use as input URLs list.")

	flag.Parse()
	// Types of arguments: Flags (e.g. -B(,) options/args with values, positional arguments
	// https://betterdev.blog/command-line-arguments-anatomy-explained/
	fmt.Println(flag.Arg(0))
	fmt.Println(isBackground, localFilename, localPath, inputFile, rateLimit)
	fmt.Println(len(flag.Args()))
	// finished parsing arguments
	//Example of very sophisticated CLI (command line interface tool) written in Go: https://github.com/kubernetes/kubectl

	//-------------------------------

	// Implementation of "-B" / Background option - redirecting the STDOUT to file
	if isBackground {
		fmt.Println("Output will be written to \"wget-log\".")
		fh, err := os.OpenFile("wget-log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666) // Read https://pkg.go.dev/os#OpenFile and https://pkg.go.dev/os#pkg-constants
		if err != nil {
			panic(err)
		}
		defer func() {
			if err := fh.Close(); err != nil {
				panic(err)
			}
		}()
		os.Stdout = fh
	}
	// Done

	//-------------------------------

	// Implement handling of -O (local file name) and -P (local path) flags but constructing the file handler inputs
	inputURL := flag.Arg(0)
	// Does our URL contain a file name, as in https://pbs.twimg.com/media/EMtmPFLWkAA8CIS.jpg?
	// Or not, as in https://adam-jerusalem.nd.edu/?
	// How do we check? Googled and found this: https://github.com/peeyushsrj/golang-snippets/blob/master/files/filename-from-url.go

	u, err := url.ParseRequestURI(inputURL) // Use http/url standard libray to parse the URL to sections - https://pkg.go.dev/net/url#ParseRequestURI , but why not https://pkg.go.dev/net/url#Parse
	if err != nil {
		panic(err)
	}
	// What is a URL? https://en.wikipedia.org/wiki/URL , https://blog.hubspot.com/marketing/parts-url, https://datatracker.ietf.org/doc/html/rfc1738 / https://datatracker.ietf.org/doc/html/rfc3986
	x, _ := url.QueryUnescape(u.EscapedPath())
	// Printing out the value we got as filename from the URL:
	URLFilename := filepath.Base(x)
	fmt.Println("URL filename:", URLFilename)

	// NOW - let's do it properly !!!!!!!!!!!
	if localFilename == "" { // if we get a local filename from the user, we just use it, if not - we need to decide what to use:
		u, err := url.ParseRequestURI(inputURL)
		if err != nil {
			panic(err)
		}
		x, _ := url.QueryUnescape(u.EscapedPath())
		URLFilename := filepath.Base(x)
		if URLFilename != "." && URLFilename != "/" { // I tried using the URL parsing and found out that when no filename is present it will sometimes return . and sometimes /
			localFilename = URLFilename
		} else {
			localFilename = "index.html" // If we didn't get a filename form the user (-O) and we didn't find an explicit filename in the URL, let's just call the file index.html.
		}
	}

	fmt.Println("Final local filename:", localFilename)

	if len(localPath) > 0 {
		if info, err := os.Stat(localPath); err != nil || !info.IsDir() {
			fmt.Println("The local path provided is invalid, or isn't a directory", localPath, err)
			os.Exit(1)
		}
	} else {
		localPath = "./"
	}
	fmt.Println("Final local path:", localPath)
	fmt.Println("File to open for saving:", localPath+localFilename)

	if inputFile != "" {
		file, err := os.Open(inputFile)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		var urls []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			urls = append(urls, scanner.Text())
		}
		fmt.Println(urls)
		//return urls, scanner.Err()
	}

	// Done
	//-------------------------------

	// Handle rate limit
	// First let's parse the rate limit input and make sure we have it in KB as the argument suppports KB as the lowest unit
	var rateLimitKb int

	if rateLimit != "" {
		switch strings.ToLower(string(rateLimit[len(rateLimit)-1:])) {
		case "k":
			rateLimitKb, err = strconv.Atoi(rateLimit[0 : len(rateLimit)-1])
			if err != nil {
				panic(err)
			}
		case "m":
			rateLimitMb, err := strconv.Atoi(rateLimit[0 : len(rateLimit)-1])
			if err != nil {
				panic(err)
			}
			rateLimitKb = rateLimitMb * 1024
		default:
			fmt.Println("Can't parse rate limit: ", rateLimit, ", please use digits and k or m suffix.")
			os.Exit(1)
		}
	}
	fmt.Println("Rate limit is: ", rateLimitKb, "KB/s")

	var datachunk int64 = 1024 * int64(rateLimitKb) // Bytes
	var timelapse time.Duration = 1                 //per seconds

	resp, _ := http.Get(inputURL)
	fmt.Println(resp.StatusCode)
	lastStart := time.Now()
	for range time.Tick(timelapse * time.Second) { // The Tick is providing an iteration of the for loop every 1 second
		fmt.Println(time.Since(lastStart)) //print to test Tick
		lastStart = time.Now()
		_, err := io.CopyN(io.Discard, resp.Body, datachunk) // each iteration we retrieve the max bytes we are allowed by the limit - https://pkg.go.dev/io#CopyN
		if err != nil {
			break
		}

	resp, _ := http.Get(inputURL)
	if resp.StatusCode == 200 && strings.Contains(resp.Header.Get("Content-Type"), "text/html") {

		fmt.Println("Content type is text/html")
		tokenizer := html.NewTokenizer(resp.Body)
		for {
			tokenType := tokenizer.Next()
			if tokenType == html.ErrorToken {
				break
			}
			parsedURL, err := url.Parse(inputURL)
			if err != nil {
				panic(err)
			}
			token := tokenizer.Token() // https://pkg.go.dev/golang.org/x/net/html#Token
			for indx, attr := range token.Attr {
				// I decided only look for html tags with attribute "src" as this is the attribute that points to the additional files needed for img and script tags,
				// I also put a condition to only process .js files:
				if attr.Key == "src" && strings.Contains(attr.Val, "js") { //&& strings.Contains(attr.Val, strings.TrimPrefix(parsedURL.Host, "www.")) {
					// fmt.Println("T:", token.Type, "A:", token.DataAtom, "D:", token.Data, "Attr:", token.Attr)
					// parsedURL.Scheme = ""
					// parsedURL.Host = "."
					//attr.Val = parsedURL.Path
					token.Attr[indx].Val = strings.ReplaceAll(attr.Val, "http", "ptth")
					fmt.Println("Token String:", token.String(), "; URL: ", parsedURL)
				}
			}
		}

	}

	//-------------------------------
	// Check inputs validity
	if len(flag.Args()) < 1 && len(inputFile) < 1 {
		fmt.Println("Please provide a URL or use the -i flag to provide an inputs file.")
		flag.Usage()
		os.Exit(1)
	}


	if info, err := os.Stat(inputFile); len(inputFile) > 0 && (errors.Is(err, os.ErrNotExist) || info.IsDir()) {
		fmt.Println("Error: File", inputFile, "not found, or it's a directory.")
	}

}

func handleURL(inputURL string) {
	resp, err := http.Get(inputURL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		fmt.Println("Content type is text/html")
	}

	for k, v := range resp.Header {
		fmt.Println(k, v)
	}
	fmt.Println(resp.Status, resp.StatusCode, resp.ContentLength)
	bodyReader := bufio.NewScanner(resp.Body)
	for bodyReader.Scan() {
		fmt.Println(bodyReader.Text())
		fmt.Println("------")

	}

}
}