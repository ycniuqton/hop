// Command hop moves files between your own machines through one S3 bucket.
package main

import (
	"os"

	"github.com/ycniuqton/hop/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(int(cli.Execute(version)))
}
