package jbd2

import (
	"os"
	"path"
)

var (
	assetsPath = path.Join(os.Getenv("GOPATH"), "src", "github.com", "dsoprea", "go-ext4", "jbd2", "assets")
)
