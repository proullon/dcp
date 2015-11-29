package main

import (
	"flag"
	"fmt"

	"github.com/proullon/dcp"
)

func main() {
	var port = flag.String("port", "", "port to listen on")
	var uri = flag.String("uri", "", "destination endpoint")
	var dir = flag.String("directory", "", "directory to manipulate")
	var tag = flag.String("tag", "", "directory tag")

	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		usage()
		return
	}

	dcp.Verbose = true

	switch args[0] {
	case "listen":
		listen(port, dir)
	case "copyto":
		copyto(dir, uri, tag)
	case "diff":
		diff(dir, uri, tag)
	default:
		usage()
	}
}

func diff(dir *string, uri *string, tag *string) {
	if dir == nil || *dir == "" {
		fmt.Printf("Directory not specified\n")
		return
	}

	if uri == nil || *uri == "" {
		fmt.Printf("Target endpoint not specified\n")
		return
	}

	if tag == nil || *tag == "" {
		fmt.Printf("Directory tag not specified\n")
		return
	}

	diff, err := dcp.GetDiff(*uri, *dir, *tag)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	if len(diff.Modified) > 0 {
		fmt.Printf("Modified files:\n")
		for _, f := range diff.Modified {
			fmt.Printf("- %s\n", f.Path)
		}
	}

	if len(diff.ClientNew) > 0 {
		fmt.Printf("New files here:\n")
		for _, f := range diff.ClientNew {
			fmt.Printf("- %s\n", f.Path)
		}
	}

	if len(diff.ServerNew) > 0 {
		fmt.Printf("New files on host:\n")
		for _, f := range diff.ServerNew {
			fmt.Printf("- %s\n", f.Path)
		}
	}
}

func copyto(dir *string, uri *string, tag *string) {
	if dir == nil || *dir == "" {
		fmt.Printf("Directory not specified\n")
		return
	}

	if uri == nil || *uri == "" {
		fmt.Printf("Target endpoint not specified\n")
		return
	}

	if tag == nil || *tag == "" {
		fmt.Printf("Directory tag not specified\n")
		return
	}

	err := dcp.CopyTo(*uri, *dir, *tag)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
}

func listen(port *string, dir *string) {
	if port == nil || *port == "" {
		fmt.Printf("Port not specified\n")
		return
	}

	if dir == nil || *dir == "" {
		fmt.Printf("Base directory not specified\n")
		return
	}

	err := dcp.Host(":"+*port, *dir)
	if err != nil {
		fmt.Printf("Cannot host: %s\n", err)
	}
}

func usage() {
	fmt.Printf("Usage:\n")
	fmt.Printf("\t-port=PORT -directory=DIR listen\n")
	fmt.Printf("\t-uri=URI -directory=DIR -tag copyto\n")
	fmt.Printf("\t-uri=URI -directory=DIR -tag copyfrom\n")
}
